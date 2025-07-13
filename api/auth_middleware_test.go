package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestAuthMiddleware(t *testing.T) {
	server := &server{
		tracer:       noop.NewTracerProvider().Tracer("test"),
		sessionStore: sessions.NewCookieStore([]byte("test-key-32-bytes-long-for-tests")),
	}

	// Handler to check if user ID is set in context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserIDFromContext(r.Context())
		if !ok {
			t.Error("UserID not found in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if userID != 123 {
			t.Errorf("Expected UserID 123, got %d", userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("session auth successful", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Create session with valid user data
		session, _ := server.sessionStore.Get(req, "user-session")
		session.Values["user"] = SessionData{
			UserID:    123,
			Email:     "test@example.com",
			Name:      "Test User",
			Provider:  "authentik",
			IssuedAt:  time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		}
		session.Save(req, rec)

		// Copy cookies from recorder to new request
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}

		rec = httptest.NewRecorder()
		middleware := server.authMiddleware(testHandler)
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("session auth expired", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		// Create session with expired user data
		session, _ := server.sessionStore.Get(req, "user-session")
		session.Values["user"] = SessionData{
			UserID:    123,
			Email:     "test@example.com",
			Name:      "Test User",
			Provider:  "authentik",
			IssuedAt:  time.Now().Add(-2 * time.Hour),
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		session.Save(req, rec)

		// Copy cookies from recorder to new request
		for _, cookie := range rec.Result().Cookies() {
			req.AddCookie(cookie)
		}

		rec = httptest.NewRecorder()
		middleware := server.authMiddleware(testHandler)
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("no auth provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware := server.authMiddleware(testHandler)
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid bearer token format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "InvalidToken")
		rec := httptest.NewRecorder()

		middleware := server.authMiddleware(testHandler)
		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
	})
}
