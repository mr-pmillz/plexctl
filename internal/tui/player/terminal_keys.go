package player

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

func readPlaybackKeys(input io.Reader) (<-chan string, func(), error) {
	f, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil, nil, fmt.Errorf("video playback requires terminal input")
	}
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return nil, nil, err
	}
	r, err := cancelreader.NewReader(f)
	if err != nil {
		_ = term.Restore(int(f.Fd()), state)
		return nil, nil, err
	}
	keys := make(chan string, 16)
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		defer close(keys)
		buf := make([]byte, 256)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				select {
				case keys <- string(buf[:n]):
				case <-stop:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return keys, func() {
		close(stop)
		r.Cancel()
		<-done
		_ = r.Close()
		_ = term.Restore(int(f.Fd()), state)
	}, nil
}

var escapeKeys = map[string]string{
	"\x1b[A": "UP", "\x1b[B": "DOWN", "\x1b[C": "RIGHT", "\x1b[D": "LEFT",
	"\x1bOA": "UP", "\x1bOB": "DOWN", "\x1bOC": "RIGHT", "\x1bOD": "LEFT",
	"\x1b[H": "HOME", "\x1b[F": "END", "\x1b[5~": "PGUP", "\x1b[6~": "PGDWN",
	"\x1b[3~": "DEL", "\x1b[2~": "INS",
}

// playbackKeys preserves incomplete escape/UTF-8 sequences between pipe reads.
func playbackKeys(input string, flush bool) (keys []string, pending string) {
	for input != "" {
		if strings.HasPrefix(input, "\x1b[<") {
			end := strings.IndexAny(input[3:], "Mm")
			if end < 0 {
				if !flush && len(input) <= 64 {
					return keys, input
				}
				return keys, "" // Drop malformed reports without sending their digits to mpv.
			}
			end += 3
			if key := playbackMouseKey(input[3:end], input[end] == 'M'); key != "" {
				keys = append(keys, key)
			}
			input = input[end+1:]
			continue
		}
		matched, partial := false, false
		if input[0] == '\x1b' {
			for sequence, key := range escapeKeys {
				if strings.HasPrefix(input, sequence) {
					keys = append(keys, key)
					input = input[len(sequence):]
					matched = true
					break
				}
				partial = partial || strings.HasPrefix(sequence, input)
			}
			if matched {
				continue
			}
			if partial && !flush {
				return keys, input
			}
		}
		if !utf8.FullRuneInString(input) && !flush {
			return keys, input
		}
		r, size := utf8.DecodeRuneInString(input)
		input = input[size:]
		switch r {
		case ' ':
			keys = append(keys, "SPACE")
		case '\r', '\n':
			keys = append(keys, "ENTER")
		case '\t':
			keys = append(keys, "TAB")
		case '\x1b':
			keys = append(keys, "ESC")
		case '\x03':
			keys = append(keys, "q")
		case '\x7f', '\b':
			keys = append(keys, "BS")
		default:
			if r >= 32 && r != utf8.RuneError {
				keys = append(keys, string(r))
			}
		}
	}
	return keys, ""
}

func playbackMouseKey(report string, pressed bool) string {
	fields := strings.Split(report, ";")
	if !pressed || len(fields) != 3 {
		return ""
	}
	button, err := strconv.Atoi(fields[0])
	x, xErr := strconv.Atoi(fields[1])
	y, yErr := strconv.Atoi(fields[2])
	if err != nil || xErr != nil || yErr != nil || x <= 0 || y <= 0 || button < 0 || button&32 != 0 || button >= 128 {
		return ""
	}
	if button&64 != 0 {
		switch button & 3 {
		case 0, 3:
			return "RIGHT"
		case 1, 2:
			return "LEFT"
		}
	}
	switch button & 3 {
	case 0:
		return "SPACE"
	case 2:
		return "q"
	}
	return ""
}
