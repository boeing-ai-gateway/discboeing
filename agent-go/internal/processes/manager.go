package processes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const ringBufferSize = 512 * 1024
const processSessionEnv = "DISCOBOT_PROCESS_SESSION_ID"

type managedSession struct {
	mu        sync.Mutex
	session   Session
	stream    Stream
	log       *outputLog
	buf       *ringBuffer
	subs      map[chan OutputEvent]struct{}
	done      chan struct{}
	nextSeq   int64
	readWG    sync.WaitGroup
	closeOnce sync.Once
}

// Manager supervises user exec and service process sessions.
type Manager struct {
	mu             sync.RWMutex
	sessions       map[string]*managedSession
	reuse          map[string]string
	defaultWorkDir string
}

// NewManager creates a process manager that uses defaultWorkDir when a start
// request does not specify WorkDir.
func NewManager(defaultWorkDir string) *Manager {
	m := &Manager{
		sessions:       map[string]*managedSession{},
		reuse:          map[string]string{},
		defaultWorkDir: defaultWorkDir,
	}
	m.cleanupAbandoned()
	return m
}

func (m *Manager) Capabilities() Capabilities {
	return platformCapabilities()
}

func (m *Manager) Start(ctx context.Context, req CreateRequest) (*Session, error) {
	if req.Kind == "" {
		req.Kind = KindUserExec
	}
	if req.ReuseKey != "" {
		if s := m.findReusable(req.ReuseKey); s != nil {
			return s, nil
		}
	}
	if len(req.Cmd) == 0 {
		req.Cmd = defaultShell(req.User)
	}
	homeWorkDir := ""
	if req.HomeDir {
		homeDir, err := homeDirForUser(req.User)
		if err != nil {
			return nil, err
		}
		req.WorkDir = homeDir
		homeWorkDir = homeDir
	} else if req.WorkDir == "" {
		req.WorkDir = m.defaultWorkDir
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}
	if req.Cols <= 0 {
		req.Cols = 80
	}

	id := "exec_" + randomID()
	if req.Env == nil {
		req.Env = map[string]string{}
	} else {
		env := make(map[string]string, len(req.Env)+1)
		maps.Copy(env, req.Env)
		req.Env = env
	}
	if homeWorkDir != "" {
		req.Env["HOME"] = homeWorkDir
	}
	req.Env[processSessionEnv] = id
	log, err := newOutputLog(id, req.TTY, req.LogDir, req.LogPath)
	if err != nil {
		return nil, err
	}
	stream, proc, err := startPlatform(ctx, req)
	if err != nil {
		_ = log.Close()
		return nil, err
	}

	sess := Session{
		ID:        id,
		Kind:      req.Kind,
		Name:      req.Name,
		ReuseKey:  req.ReuseKey,
		Cmd:       append([]string(nil), req.Cmd...),
		WorkDir:   req.WorkDir,
		User:      req.User,
		TTY:       req.TTY,
		Status:    StatusRunning,
		PID:       proc.pid,
		PGID:      proc.pgid,
		StartedAt: time.Now().UTC(),
		LogPath:   log.logPath,
		Metadata:  req.Metadata,
	}
	managed := &managedSession{
		session: sess,
		stream:  stream,
		log:     log,
		buf:     newRingBuffer(ringBufferSize),
		subs:    map[chan OutputEvent]struct{}{},
		done:    make(chan struct{}),
	}
	m.mu.Lock()
	m.sessions[id] = managed
	if req.ReuseKey != "" {
		m.reuse[req.ReuseKey] = id
	}
	m.mu.Unlock()
	managed.persist()
	managed.readWG.Go(func() {
		m.readLoop(managed, "stdout", stream)
	})
	if stderr := stream.Stderr(); stderr != nil {
		managed.readWG.Go(func() {
			m.readLoop(managed, "stderr", stderr)
		})
	}
	go m.waitLoop(context.Background(), managed)
	return managed.snapshot(), nil
}

func (m *Manager) List(kind Kind) []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Session, 0, len(m.sessions))
	for _, managed := range m.sessions {
		s := managed.snapshot()
		if kind == "" || s.Kind == kind {
			out = append(out, *s)
		}
	}
	return out
}

func (m *Manager) Get(id string) (*Session, error) {
	managed := m.get(id)
	if managed == nil {
		return nil, ErrNotFound
	}
	return managed.snapshot(), nil
}

func (m *Manager) Attach(id string) (Stream, <-chan OutputEvent, func(), error) {
	managed := m.get(id)
	if managed == nil {
		return nil, nil, nil, ErrNotFound
	}
	ch := make(chan OutputEvent, 512)
	managed.mu.Lock()
	if snapshot := managed.buf.snapshot(); len(snapshot) > 0 {
		ch <- nowEvent(outputType(managed.session.TTY, "stdout"), string(snapshot))
	}
	if managed.subs == nil {
		close(ch)
	} else {
		managed.subs[ch] = struct{}{}
	}
	managed.mu.Unlock()
	return managed.stream, ch, func() { managed.unsubscribe(ch) }, nil
}

func (m *Manager) Subscribe(id string) (<-chan OutputEvent, func(), <-chan struct{}, error) {
	managed := m.get(id)
	if managed == nil {
		return nil, nil, nil, ErrNotFound
	}
	ch := make(chan OutputEvent, 512)
	managed.mu.Lock()
	if managed.subs == nil {
		close(ch)
	} else {
		managed.subs[ch] = struct{}{}
	}
	managed.mu.Unlock()
	return ch, func() { managed.unsubscribe(ch) }, managed.done, nil
}

func (m *Manager) Output(id string) ([]OutputEvent, error) {
	return m.Events(id, EventQuery{})
}

// Events returns the persisted event log for a session, optionally filtered.
func (m *Manager) Events(id string, query EventQuery) ([]OutputEvent, error) {
	managed := m.get(id)
	if managed == nil {
		return nil, ErrNotFound
	}
	path := filepath.Join(filepath.Dir(managed.session.LogPath), "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := bytesLines(data)
	events := make([]OutputEvent, 0, len(lines))
	for _, line := range lines {
		var event OutputEvent
		if err := jsonUnmarshal(line, &event); err == nil {
			events = append(events, event)
		}
	}
	return filterEvents(events, query), nil
}

func (m *Manager) Resize(ctx context.Context, id string, rows, cols int) error {
	managed := m.get(id)
	if managed == nil {
		return ErrNotFound
	}
	return managed.stream.Resize(ctx, rows, cols)
}

func (m *Manager) CloseWrite(id string) error {
	managed := m.get(id)
	if managed == nil {
		return ErrNotFound
	}
	return managed.stream.CloseWrite()
}

func (m *Manager) Kill(id string) error {
	managed := m.get(id)
	if managed == nil {
		return ErrNotFound
	}
	managed.mu.Lock()
	managed.session.Status = StatusKilling
	managed.persistLocked()
	managed.mu.Unlock()
	return killPlatform(managed.stream, managed.session.PID, managed.session.PGID)
}

func (m *Manager) readLoop(managed *managedSession, streamType string, r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			eventType := outputType(managed.session.TTY, streamType)
			managed.emit(nowEvent(eventType, data))
		}
		if err != nil {
			return
		}
	}
}

func (m *Manager) waitLoop(ctx context.Context, managed *managedSession) {
	managed.readWG.Wait()
	code, err := managed.stream.Wait(ctx)
	if err != nil && code < 0 {
		code = 1
	}
	exited := time.Now().UTC()
	managed.mu.Lock()
	managed.session.ExitCode = new(code)
	if managed.session.Status == StatusKilling {
		managed.session.Status = StatusKilled
	} else {
		managed.session.Status = StatusExited
	}
	managed.session.ExitedAt = &exited
	managed.persistLocked()
	managed.mu.Unlock()
	managed.emit(OutputEvent{Type: "exit", ExitCode: new(code), Timestamp: exited})
	managed.closeOnce.Do(func() {
		managed.mu.Lock()
		for ch := range managed.subs {
			close(ch)
		}
		managed.subs = nil
		managed.mu.Unlock()
		close(managed.done)
		_ = managed.log.Close()
	})
}

func (m *Manager) get(id string) *managedSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *Manager) findReusable(key string) *Session {
	m.mu.RLock()
	id := m.reuse[key]
	managed := m.sessions[id]
	m.mu.RUnlock()
	if managed == nil {
		return nil
	}
	s := managed.snapshot()
	if s.Status == StatusRunning || s.Status == StatusStarting {
		return s
	}
	return nil
}

func (m *Manager) cleanupAbandoned() {
	_ = os.MkdirAll(dataDir(), 0o755)
	entries, err := os.ReadDir(dataDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataDir(), entry.Name(), "session.json")
		var s Session
		if readJSON(path, &s) != nil || s.Status != StatusRunning {
			continue
		}
		if !shouldCleanupAbandoned(s) {
			now := time.Now().UTC()
			s.Status = StatusKilled
			s.ExitedAt = &now
			_ = writeJSON(path, s)
			continue
		}
		cleanupPlatform(s.PID, s.PGID)
		now := time.Now().UTC()
		s.Status = StatusKilled
		s.ExitedAt = &now
		_ = writeJSON(path, s)
	}
}

func (s *managedSession) snapshot() *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.session
	session.Cmd = append([]string(nil), s.session.Cmd...)
	return &session
}

func (s *managedSession) emit(event OutputEvent) {
	s.mu.Lock()
	if event.Seq == 0 {
		s.nextSeq++
		event.Seq = s.nextSeq
	}
	s.log.WriteEvent(event)
	if event.Type == "stdout" || event.Type == "stderr" || event.Type == "output" {
		s.buf.write([]byte(event.Data))
	}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *managedSession) unsubscribe(ch chan OutputEvent) {
	s.mu.Lock()
	_, ok := s.subs[ch]
	if ok {
		delete(s.subs, ch)
	}
	s.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (s *managedSession) persist() { s.mu.Lock(); defer s.mu.Unlock(); s.persistLocked() }
func (s *managedSession) persistLocked() {
	_ = writeJSON(filepath.Join(filepath.Dir(s.session.LogPath), "session.json"), s.session)
}

func outputType(tty bool, streamType string) string {
	if tty {
		return "output"
	}
	return streamType
}

func filterEvents(events []OutputEvent, query EventQuery) []OutputEvent {
	out := events[:0]
	for _, event := range events {
		if query.After != nil && event.Seq <= *query.After {
			continue
		}
		if query.Since != nil && !event.Timestamp.After(*query.Since) {
			continue
		}
		out = append(out, event)
	}
	if query.Limit > 0 && len(out) > query.Limit {
		out = out[len(out)-query.Limit:]
	}
	return out
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func defaultShell(_ string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe"}
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		if info, err := os.Stat(shell); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return []string{shell, "-l"}
		}
	}
	if _, err := os.Stat("/bin/bash"); err == nil {
		return []string{"/bin/bash", "-l"}
	}
	return []string{"/bin/sh"}
}

func homeDirForUser(targetUser string) (string, error) {
	targetUser = strings.TrimSpace(targetUser)
	if targetUser == "" || isCurrentUserTarget(targetUser) {
		return os.UserHomeDir()
	}

	userPart, _, _ := strings.Cut(targetUser, ":")
	if userPart == "" {
		return os.UserHomeDir()
	}
	if isNumericUser(userPart) {
		u, err := user.LookupId(userPart)
		if err != nil {
			return "", err
		}
		return u.HomeDir, nil
	}
	u, err := user.Lookup(userPart)
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

func isNumericUser(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}
