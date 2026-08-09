// Package session provides temporary opaque-cookie identity for the hackathon demo.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"
)

// CookieName is the opaque demo-session cookie name.
const CookieName = "swap_chain_session"

// ErrNoSession indicates that a request has no active demo session.
var ErrNoSession = errors.New("demo session is not established")

// Session identifies the selected demo user and expiration time.
type Session struct {
	UserID    int64
	ExpiresAt time.Time
	token     string
}

type storedSession struct {
	Session
}

// Manager stores short-lived opaque demo sessions in process memory.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]storedSession
	ttl      time.Duration
	secure   bool
	now      func() time.Time
}

// NewManager creates a session manager with configured cookie attributes.
func NewManager(ttl time.Duration, secure bool) *Manager {
	return &Manager{
		sessions: make(map[string]storedSession),
		ttl:      ttl,
		secure:   secure,
		now:      time.Now,
	}
}

// Create establishes a new opaque session for a selected demo user.
func (m *Manager) Create(userID int64) (Session, string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return Session{}, "", err
	}

	token := base64.RawURLEncoding.EncodeToString(random)
	created := storedSession{
		Session: Session{UserID: userID, ExpiresAt: m.now().UTC().Add(m.ttl), token: token},
	}

	m.mu.Lock()
	m.sessions[token] = created
	m.mu.Unlock()

	return created.Session, m.cookie(token, created.ExpiresAt), nil
}

// Revoke removes an active session and returns a cookie that clears it in the browser.
func (m *Manager) Revoke(current Session) string {
	m.mu.Lock()
	delete(m.sessions, current.token)
	m.mu.Unlock()

	return (&http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}).String()
}

// Resolve loads a non-expired session from a request cookie.
func (m *Manager) Resolve(request *http.Request) (Session, error) {
	cookie, err := request.Cookie(CookieName)
	if err != nil {
		return Session{}, ErrNoSession
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stored, ok := m.sessions[cookie.Value]
	if !ok {
		return Session{}, ErrNoSession
	}
	if !m.now().Before(stored.ExpiresAt) {
		delete(m.sessions, cookie.Value)
		return Session{}, ErrNoSession
	}

	return stored.Session, nil
}

// Middleware adds a valid demo session to the request context when present.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		current, err := m.Resolve(request)
		if err == nil {
			request = request.WithContext(context.WithValue(request.Context(), sessionContextKey{}, current))
		}
		next.ServeHTTP(writer, request)
	})
}

// Current returns the demo session attached to the request context.
func Current(ctx context.Context) (Session, bool) {
	current, ok := ctx.Value(sessionContextKey{}).(Session)
	return current, ok
}

func (m *Manager) cookie(token string, expiresAt time.Time) string {
	return (&http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}).String()
}

type sessionContextKey struct{}
