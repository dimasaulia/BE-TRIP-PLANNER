package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type dailyWriter struct {
	dir         string
	mu          sync.Mutex
	currentDate string
	file        *os.File
}

func newDailyWriter(dir string) *dailyWriter {
	return &dailyWriter{
		dir: dir,
	}
}

func (w *dailyWriter) Write(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(time.Now()); err != nil {
		return 0, err
	}

	return w.file.Write(payload)
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	return w.file.Close()
}

func (w *dailyWriter) rotateIfNeeded(now time.Time) error {
	date := now.Format("2006-01-02")
	if w.file != nil && w.currentDate == date {
		return nil
	}

	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}

	if w.file != nil {
		_ = w.file.Close()
	}

	file, err := os.OpenFile(filepath.Join(w.dir, "app-"+date+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	w.file = file
	w.currentDate = date
	return nil
}
