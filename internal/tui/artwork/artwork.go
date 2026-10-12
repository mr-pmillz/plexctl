// Package artwork renders native Kitty posters using Unicode placeholders.
// Placeholders remain ordinary cells to Bubble Tea and tmux, so scrolling,
// clipping, borders, and overlays work without an independent image layer.
package artwork

import (
	"fmt"
	"image"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi/kitty"
	gopixels "github.com/saran13raj/go-pixels"
	"github.com/ygelfand/plexctl/internal/plex"
	"github.com/ygelfand/plexctl/internal/termgraphics"
	"golang.org/x/image/draw"
)

var posters = struct {
	sync.Mutex
	enabled bool
	dir     string
	nextID  int
	views   map[string]string
	files   map[int]string
	wanted  map[int]bool
	sent    map[int]uint64
	clock   uint64
}{}

// Open enables native posters only after a successful Kitty protocol probe.
// Image payloads live in a temporary directory, rather than accumulating in RAM
// or persisting terminal-specific image IDs in the old text-poster cache.
func Open(output *os.File, enabled bool) (*Output, error) {
	posters.Lock()
	defer posters.Unlock()
	posters.enabled = enabled
	posters.views = make(map[string]string)
	posters.files = make(map[int]string)
	posters.wanted = make(map[int]bool)
	posters.sent = make(map[int]uint64)
	posters.nextID = 1 + rand.IntN(1<<22)
	if enabled {
		dir, err := os.MkdirTemp("", "plexctl-posters-")
		if err != nil {
			posters.enabled = false
			return nil, err
		}
		posters.dir = dir
	}
	return &Output{file: output}, nil
}

func key(ratingKey string, width int) string {
	// Callers include the server and credential namespace in the artwork key.
	return fmt.Sprintf("%s/%d", ratingKey, width)
}

func Cached(ratingKey string, width int) (string, bool) {
	posters.Lock()
	defer posters.Unlock()
	if posters.enabled {
		view, ok := posters.views[key(ratingKey, width)]
		return view, ok
	}
	return plex.GetCachedPoster(ratingKey, width)
}

func Render(ratingKey string, img image.Image, width, height int) (string, error) {
	posters.Lock()
	enabled, dir := posters.enabled, posters.dir
	id := posters.nextID
	posters.nextID++
	posters.Unlock()
	if !enabled {
		view, err := gopixels.FromImageStream(img, width, 0, "halfcell", true)
		if err == nil && ratingKey != "" {
			plex.SetCachedPoster(ratingKey, width, view)
		}
		return view, err
	}
	if width <= 0 || width > 255 {
		return "", fmt.Errorf("invalid poster width: %d", width)
	}
	if height <= 0 {
		height = max(1, (width*img.Bounds().Dy()/img.Bounds().Dx()+1)/2)
	}
	height = min(height, 255)
	// Retain hundreds of pixels of detail, including on HiDPI displays, while
	// bounding transport and terminal texture memory for very large originals.
	if longest := max(img.Bounds().Dx(), img.Bounds().Dy()); longest > 768 {
		resized := image.NewRGBA(image.Rect(0, 0,
			max(1, img.Bounds().Dx()*768/longest), max(1, img.Bounds().Dy()*768/longest)))
		draw.CatmullRom.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Over, nil)
		img = resized
	}
	path := filepath.Join(dir, strconv.Itoa(id))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	err = kitty.EncodeGraphics(f, img, &kitty.Options{
		Action: kitty.TransmitAndPut, Quite: 2, ID: id, Format: kitty.PNG,
		Transmission: kitty.Direct, Chunk: true, ChunkFormatter: termgraphics.Wrap,
		Columns: width, Rows: height, VirtualPlacement: true,
	})
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	view := placeholders(id, width, height)
	posters.Lock()
	posters.files[id] = path
	posters.views[key(ratingKey, width)] = view
	posters.Unlock()
	return view, nil
}

func placeholders(id, columns, rows int) string {
	var s strings.Builder
	for y := range rows {
		if y > 0 {
			s.WriteByte('\n')
		}
		fmt.Fprintf(&s, "\x1b[38;2;%d;%d;%dm\x1b[59m", id>>16&255, id>>8&255, id&255)
		for x := range columns {
			s.WriteRune(kitty.Placeholder)
			s.WriteRune(kitty.Diacritic(y))
			s.WriteRune(kitty.Diacritic(x))
			s.WriteRune(kitty.Diacritic(0))
		}
		s.WriteString("\x1b[39m")
	}
	return s.String()
}

var imageColor = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)

// Frame records images in the final, clipped/composited view. The output writer
// sends their payloads before the next screen update, on the renderer's own
// goroutine. Background image fetches never write directly to the terminal.
func Frame(view string) string {
	posters.Lock()
	defer posters.Unlock()
	if !posters.enabled {
		return view
	}
	wanted := make(map[int]bool)
	for _, m := range imageColor.FindAllStringSubmatch(view, -1) {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		id := r<<16 | g<<8 | b
		if _, ok := posters.files[id]; ok {
			wanted[id] = true
		}
	}
	posters.wanted = wanted
	return view
}

type Output struct{ file *os.File }

func (o *Output) Fd() uintptr { return o.file.Fd() }

func (o *Output) Read(p []byte) (int, error) { return o.file.Read(p) }

// TerminalFile keeps mpv attached to a real TTY when Bubble Tea lends it output.
func (o *Output) TerminalFile() *os.File { return o.file }

func (o *Output) Write(p []byte) (int, error) {
	posters.Lock()
	defer posters.Unlock()
	if strings.Contains(string(p), "\x1b[?1049l") {
		// An external player may clear graphics while Bubble Tea is suspended.
		clear(posters.sent)
		return o.file.Write(p)
	}
	for id := range posters.wanted {
		posters.clock++
		if _, ok := posters.sent[id]; !ok {
			data, err := os.ReadFile(posters.files[id])
			if err != nil {
				return 0, err
			}
			if _, err := o.file.Write(data); err != nil {
				return 0, err
			}
		}
		posters.sent[id] = posters.clock
	}
	// Bound terminal texture memory; inactive posters can be uploaded again.
	for len(posters.sent) > max(64, len(posters.wanted)) {
		oldest, age := 0, ^uint64(0)
		for id, used := range posters.sent {
			if !posters.wanted[id] && used < age {
				oldest, age = id, used
			}
		}
		if oldest == 0 {
			break
		}
		_, _ = io.WriteString(o.file, deleteImage(oldest))
		delete(posters.sent, oldest)
	}
	return o.file.Write(p)
}

func deleteImage(id int) string {
	return termgraphics.Wrap(fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2;\x1b\\", id))
}

func (o *Output) Close() error {
	posters.Lock()
	defer posters.Unlock()
	for id := range posters.files {
		_, _ = io.WriteString(o.file, deleteImage(id))
	}
	posters.enabled = false
	clear(posters.wanted)
	clear(posters.files)
	if posters.dir != "" {
		return os.RemoveAll(posters.dir)
	}
	return nil
}
