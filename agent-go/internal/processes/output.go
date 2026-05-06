package processes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type outputLog struct {
	dir     string
	mu      sync.Mutex
	out     *os.File
	stdout  *os.File
	stderr  *os.File
	events  *os.File
	logPath string
}

func newOutputLog(id string, tty bool, dir, logPath string) (*outputLog, error) {
	if dir == "" {
		dir = filepath.Join(dataDir(), id)
	}
	if logPath == "" {
		logPath = filepath.Join(dir, "output.log")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	log := &outputLog{dir: dir, logPath: logPath}
	var err error
	log.out, err = os.OpenFile(log.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if !tty {
		log.stdout, err = os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			_ = log.Close()
			return nil, err
		}
		log.stderr, err = os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			_ = log.Close()
			return nil, err
		}
	}
	log.events, err = os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		_ = log.Close()
		return nil, err
	}
	return log, nil
}

func (l *outputLog) WriteEvent(event OutputEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch event.Type {
	case "stdout":
		if l.stdout != nil {
			_, _ = l.stdout.WriteString(event.Data)
		}
		_, _ = l.out.WriteString(event.Data)
	case "stderr":
		if l.stderr != nil {
			_, _ = l.stderr.WriteString(event.Data)
		}
		_, _ = l.out.WriteString(event.Data)
	case "output":
		_, _ = l.out.WriteString(event.Data)
	}
	data, err := json.Marshal(event)
	if err == nil {
		_, _ = l.events.Write(append(data, '\n'))
	}
}

func (l *outputLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, f := range []*os.File{l.out, l.stdout, l.stderr, l.events} {
		if f != nil {
			_ = f.Close()
		}
	}
	return nil
}

func dataDir() string {
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil && dir != "" {
			return filepath.Join(dir, "Discobot", "processes")
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".discobot", "processes")
	}
	return filepath.Join(os.TempDir(), "discobot-processes")
}

func nowEvent(eventType, data string) OutputEvent {
	return OutputEvent{Type: eventType, Data: data, Timestamp: time.Now().UTC()}
}
