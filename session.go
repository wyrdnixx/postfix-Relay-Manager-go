package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
)

// Flash ist eine einmalige Benachrichtigung (wird nach dem Anzeigen gelöscht).
type Flash struct {
	Msg  string
	Type string // "ok" oder "err"
}

// Session hält den Authentifizierungszustand und optionale Flash-Meldung.
type Session struct {
	Authenticated bool
	Flash         *Flash
	mu            sync.Mutex
}

func (s *Session) getFlash() *Flash {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.Flash
	s.Flash = nil
	return f
}

func (s *Session) setFlash(f *Flash) {
	s.mu.Lock()
	s.Flash = f
	s.mu.Unlock()
}

// sessionStore ist ein In-Memory-Session-Speicher (geht beim Neustart verloren).
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

var store = &sessionStore{sessions: make(map[string]*Session)}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *sessionStore) get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func (s *sessionStore) create() (string, *Session) {
	id := newSessionID()
	sess := &Session{}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return id, sess
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func getSession(r *http.Request) (string, *Session) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return "", nil
	}
	sess := store.get(cookie.Value)
	if sess == nil {
		return "", nil
	}
	return cookie.Value, sess
}

func createSession(w http.ResponseWriter) (string, *Session) {
	id, sess := store.create()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return id, sess
}

func deleteSession(w http.ResponseWriter, id string) {
	store.delete(id)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
