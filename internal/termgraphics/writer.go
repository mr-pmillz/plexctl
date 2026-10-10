package termgraphics

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Writer wraps mpv's graphics packets for tmux, retaining ANSI cursor movement
// and screen switching in tmux itself so playback stays inside its pane.
// Packets can span arbitrary writes from os/exec's stdout pipe.
type Writer struct {
	Output io.Writer
	// ImageID assigns frames to this playback session for targeted cleanup.
	ImageID int
	pending []byte
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
		packet := ansi.TmuxPassthrough(sequence)
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
