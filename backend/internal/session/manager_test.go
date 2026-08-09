package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManagerCreatesOpaqueHttpOnlyCookieAndResolvesIt(t *testing.T) {
	manager := NewManager(time.Hour, false)
	created, setCookie, err := manager.Create(7)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(setCookie, "SameSite=Lax") {
		t.Fatalf("cookie misses security attributes: %q", setCookie)
	}

	response := httptest.NewRecorder()
	response.Header().Add("Set-Cookie", setCookie)
	if got := response.Result().Cookies()[0].Value; got == "7" {
		t.Fatalf("cookie value exposes user ID: %q", got)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	request.AddCookie(response.Result().Cookies()[0])

	resolved, err := manager.Resolve(request)
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if resolved.UserID != created.UserID {
		t.Fatalf("user ID = %d, want %d", resolved.UserID, created.UserID)
	}
}

func TestManagerRejectsUnknownCookie(t *testing.T) {
	manager := NewManager(time.Hour, false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	request.AddCookie(&http.Cookie{Name: CookieName, Value: "unknown"})

	if _, err := manager.Resolve(request); !errors.Is(err, ErrNoSession) {
		t.Fatalf("error = %v, want ErrNoSession", err)
	}
}

func TestManagerRevokesSessionAndExpiresCookie(t *testing.T) {
	manager := NewManager(time.Hour, false)
	current, setCookie, err := manager.Create(7)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	response := httptest.NewRecorder()
	response.Header().Add("Set-Cookie", setCookie)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.AddCookie(response.Result().Cookies()[0])

	clearCookie := manager.Revoke(current)
	if !strings.Contains(clearCookie, "Max-Age=0") || !strings.Contains(clearCookie, "HttpOnly") {
		t.Fatalf("clear cookie misses expected attributes: %q", clearCookie)
	}
	if _, err := manager.Resolve(request); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Resolve() after revoke error = %v, want ErrNoSession", err)
	}
}
