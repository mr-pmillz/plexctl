package termgraphics

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// Writer wraps mpv's graphics packets for tmux, retaining ANSI cursor movement
// and screen switching in tmux itself so playback stays inside its pane.
// Packets can span arbitrary writes from os/exec's stdout pipe.
type Writer struct {
	Output io.Writer
	// Direct observes graphics without wrapping them for tmux.
	Direct bool
	// ImageID assigns frames to this playback session for targeted cleanup.
	ImageID          int
	pending          []byte
	cursorX, cursorY int
	mu               sync.RWMutex
	frame            Frame
}

// Frame describes the actual Kitty placement: a cell origin and pixel size.
// Read it while playback is running to map terminal mouse reports to the image,
// including letterboxing, custom video geometry, and terminal resizing.
type Frame struct {
	X, Y, Width, Height int
}

func (w *Writer) Frame() Frame {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.frame
}

func (w *Writer) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for len(w.pending) > 0 {
		start := bytes.IndexByte(w.pending, '\x1b')
		if start < 0 {
			start = len(w.pending)
		}
		if start > 0 {
			if _, err := w.Output.Write(w.pending[:start]); err != nil {
				return 0, err
			}
			w.pending = w.pending[start:]
			continue
		}
		if len(w.pending) < 2 {
			break
		}
		if w.pending[1] == '[' {
			end := 2
			for end < len(w.pending) && (w.pending[end] < 0x40 || w.pending[end] > 0x7e) {
				end++
			}
			if end == len(w.pending) {
				if end > 4096 {
					return 0, fmt.Errorf("terminal cursor sequence exceeds 4 KiB")
				}
				break
			}
			if w.pending[end] == 'H' || w.pending[end] == 'f' {
				row, col, _ := strings.Cut(string(w.pending[2:end]), ";")
				y, _ := strconv.Atoi(row)
				x, _ := strconv.Atoi(col)
				w.cursorX, w.cursorY = max(0, x-1), max(0, y-1)
			}
			if _, err := w.Output.Write(w.pending[:end+1]); err != nil {
				return 0, err
			}
			w.pending = w.pending[end+1:]
			continue
		}
		if w.pending[1] != '_' && w.pending[1] != 'P' {
			if _, err := w.Output.Write(w.pending[:2]); err != nil {
				return 0, err
			}
			w.pending = w.pending[2:]
			continue
		}
		end := bytes.Index(w.pending[2:], []byte("\x1b\\"))
		if end < 0 {
			if len(w.pending) > 32<<20 {
				return 0, fmt.Errorf("terminal graphics packet exceeds 32 MiB")
			}
			break
		}
		end += 4
		sequence := string(w.pending[:end])
		if strings.HasPrefix(sequence, "\x1b_Ga=T,") {
			head, _, _ := strings.Cut(sequence[3:], ";")
			frame := Frame{X: w.cursorX, Y: w.cursorY}
			for field := range strings.SplitSeq(head, ",") {
				key, value, _ := strings.Cut(field, "=")
				switch key {
				case "s":
					frame.Width, _ = strconv.Atoi(value)
				case "v":
					frame.Height, _ = strconv.Atoi(value)
				}
			}
			w.mu.Lock()
			w.frame = frame
			w.mu.Unlock()
		}
		if w.ImageID != 0 && strings.HasPrefix(sequence, "\x1b_G") {
			if head, payload, ok := strings.Cut(sequence, ";"); ok {
				switch {
				case strings.Contains(head, "a=T"):
					sequence = fmt.Sprintf("%s,i=%d;%s", head, w.ImageID, payload)
				case strings.Contains(head, "a=d"):
					sequence = DeleteImage(w.ImageID)
				}
			}
		}
		packet := sequence
		if !w.Direct {
			packet = ansi.TmuxPassthrough(sequence)
		}
		if _, err := io.WriteString(w.Output, packet); err != nil {
			return 0, err
		}
		w.pending = w.pending[end:]
	}
	return len(p), nil
}

func DeleteImage(id int) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", id)
}
