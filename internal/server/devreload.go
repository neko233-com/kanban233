package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DevReloader struct {
	dir      string
	mu       sync.Mutex
	clients  map[chan string]struct{}
	seenOnce bool
	lastMod  int64
}

func NewDevReloader(dir string) *DevReloader {
	return &DevReloader{
		dir:     dir,
		clients: make(map[chan string]struct{}),
	}
}

func (d *DevReloader) Start() {
	go d.poll()
}

func (d *DevReloader) poll() {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		mod := webDirMaxModTime(d.dir)
		d.mu.Lock()
		if d.seenOnce && mod > d.lastMod {
			d.broadcastLocked("reload")
		}
		d.seenOnce = true
		d.lastMod = mod
		d.mu.Unlock()
	}
}

func (d *DevReloader) broadcastLocked(msg string) {
	for ch := range d.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (d *DevReloader) subscribe() chan string {
	ch := make(chan string, 1)
	d.mu.Lock()
	d.clients[ch] = struct{}{}
	d.mu.Unlock()
	return ch
}

func (d *DevReloader) unsubscribe(ch chan string) {
	d.mu.Lock()
	delete(d.clients, ch)
	d.mu.Unlock()
}

func (d *DevReloader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := d.subscribe()
	defer d.unsubscribe(ch)

	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func webDirMaxModTime(dir string) int64 {
	var max int64
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".html" && ext != ".js" && ext != ".css" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if t := info.ModTime().UnixNano(); t > max {
			max = t
		}
		return nil
	})
	return max
}

func staticHandlerWithDev(dir string, dev bool) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if dev {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		path := r.URL.Path
		if path == "/" || path == "" {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}
		filePath := filepath.Join(dir, filepath.Clean("/"+path))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func registerDevRoutes(mux *http.ServeMux, dir string) {
	reloader := NewDevReloader(dir)
	reloader.Start()
	mux.Handle("GET /api/dev/reload", reloader)
	log.Printf("dev mode: web hot reload enabled (watching %s)", dir)
}
