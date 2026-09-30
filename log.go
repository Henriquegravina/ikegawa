package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Henriquegravina/ikegawa/internal/typroto"
)

// localLogger appends live readings to a local CSV file, independent of the
// instrument's own save/logging memory (that's what the Memória tab is
// for). Each time the user turns logging on, StartSession opens a brand
// new file named with the current date and time (down to the second), so
// separate recordings never mix into the same file.
type localLogger struct {
	mu     sync.Mutex
	dir    string
	file   *os.File
	writer *csv.Writer
	path   string
}

// defaultLogDir returns the log folder to use when the user hasn't picked
// one of their own in Settings.
func defaultLogDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "IkeGawa", "logs"), nil
}

func newLocalLogger(dir string) (*localLogger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &localLogger{dir: dir}, nil
}

// SetDir changes the folder new sessions are written into (existing open
// files aren't moved). The folder is created if it doesn't exist yet.
func (l *localLogger) SetDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dir = dir
	return nil
}

// Dir returns the folder new sessions are currently written into.
func (l *localLogger) Dir() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dir
}

// StartSession closes any file from a previous session and opens a new one
// named ikegawa_YYYY-MM-DD_HH-MM-SS.csv, writing the header row. It
// returns the new file's path.
func (l *localLogger) StartSession(t time.Time) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closeLocked()

	path := filepath.Join(l.dir, fmt.Sprintf("ikegawa_%s.csv", t.Format("2006-01-02_15-04-05")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{"timestamp", "valor", "unidade_aux", "unidade", "funcao", "status", "bruto"}); err != nil {
		f.Close()
		return "", err
	}
	w.Flush()

	l.file = f
	l.writer = w
	l.path = path
	return path, nil
}

// Append writes one reading to the current session file. It's a no-op if
// no session is open.
func (l *localLogger) Append(t time.Time, r typroto.Reading) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer == nil {
		return nil
	}
	if err := l.writer.Write([]string{
		t.Format(time.RFC3339),
		fmt.Sprintf("%g", r.Value),
		r.AuxUnit,
		r.Unit,
		r.Function,
		r.Status,
		r.Raw,
	}); err != nil {
		return err
	}
	l.writer.Flush()
	return l.writer.Error()
}

// CurrentPath returns the path of the currently open session file, or ""
// if no session is open.
func (l *localLogger) CurrentPath() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}

// EndSession closes the current session file, if any.
func (l *localLogger) EndSession() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closeLocked()
}

func (l *localLogger) closeLocked() {
	if l.file != nil {
		l.writer.Flush()
		l.file.Close()
		l.file = nil
		l.writer = nil
		l.path = ""
	}
}

func (l *localLogger) Close() { l.EndSession() }
