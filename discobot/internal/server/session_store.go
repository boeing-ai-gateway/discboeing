package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

const sessionCookieName = "discobot_session"

type sessionStore struct {
	dir    string
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]*storedSession
}

type storedSession struct {
	View       state.View `json:"view"`
	persisting bool
	dirty      bool
	timer      *time.Timer
}

func newSessionStore(dir string, logger *slog.Logger) *sessionStore {
	return &sessionStore{
		dir:      dir,
		logger:   logger,
		sessions: map[string]*storedSession{},
	}
}

func (s *sessionStore) sessionID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && validSessionID(cookie.Value) {
		return cookie.Value
	}

	id := newSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
	})
	return id
}

func (s *sessionStore) view(id string) state.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(id).View
}

func (s *sessionStore) save(id string, update func(*state.View)) state.View {
	s.mu.Lock()
	session := s.loadLocked(id)
	view := state.NormalizeView(session.View)
	update(&view)
	session.View = view
	session.dirty = true
	s.schedulePersistLocked(id, session)
	s.mu.Unlock()
	return view
}

func (s *sessionStore) loadLocked(id string) *storedSession {
	if session := s.sessions[id]; session != nil {
		return session
	}

	session := &storedSession{View: state.DefaultView()}
	if data, err := os.ReadFile(s.file(id)); err == nil {
		var persisted storedSession
		if err := json.Unmarshal(data, &persisted); err != nil {
			s.logger.Warn("failed to decode discobot session view", "session", id, "error", err)
		} else {
			session.View = state.NormalizeView(persisted.View)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("failed to read discobot session view", "session", id, "error", err)
	}
	s.sessions[id] = session
	return session
}

func (s *sessionStore) schedulePersistLocked(id string, session *storedSession) {
	if s.dir == "" || session.persisting {
		return
	}
	if session.timer != nil {
		session.timer.Reset(time.Second)
		return
	}
	session.timer = time.AfterFunc(time.Second, func() {
		s.persist(id)
	})
}

func (s *sessionStore) persist(id string) {
	s.mu.Lock()
	session := s.loadLocked(id)
	session.timer = nil
	if !session.dirty || session.persisting {
		s.mu.Unlock()
		return
	}
	session.dirty = false
	session.persisting = true
	payload := storedSession{View: state.NormalizeView(session.View)}
	s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		s.logger.Warn("failed to create discobot session directory", "dir", s.dir, "error", err)
		s.markDirty(id)
		return
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		s.logger.Warn("failed to encode discobot session view", "session", id, "error", err)
		s.markDirty(id)
		return
	}
	file := s.file(id)
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		s.logger.Warn("failed to write discobot session view", "session", id, "error", err)
		s.markDirty(id)
		return
	}
	if err := os.Rename(tmp, file); err != nil {
		s.logger.Warn("failed to replace discobot session view", "session", id, "error", err)
		s.markDirty(id)
		return
	}

	s.mu.Lock()
	if session := s.sessions[id]; session != nil {
		session.persisting = false
		if session.dirty {
			s.schedulePersistLocked(id, session)
		}
	}
	s.mu.Unlock()
}

func (s *sessionStore) markDirty(id string) {
	s.mu.Lock()
	if session := s.sessions[id]; session != nil {
		session.persisting = false
		session.dirty = true
		s.schedulePersistLocked(id, session)
	}
	s.mu.Unlock()
}

func (s *sessionStore) file(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func validSessionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func newSessionID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))[:32]
	}
	return hex.EncodeToString(bytes[:])
}
