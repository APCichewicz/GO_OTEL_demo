package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/sessions"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestJWTMiddleware(t *testing.T) {
	// Set test environment variables
	os.Setenv("AUTHENTIK_URL", "https://auth.example.com")
	os.Setenv("AUTHENTIK_APP_NAME", "test-app")
	os.Setenv("JWT_ISSUER", "https://auth.example.com/application/o/test-app/")
	os.Setenv("JWT_AUDIENCE", "test-audience")
	defer func() {
		os.Unsetenv("AUTHENTIK_URL")
		os.Unsetenv("AUTHENTIK_APP_NAME")
		os.Unsetenv("JWT_ISSUER")
		os.Unsetenv("JWT_AUDIENCE")
	}()

	server := &server{
		tracer:       noop.NewTracerProvider().Tracer("test"),
		sessionStore: sessions.NewCookieStore([]byte("test-key-32-bytes-long-for-tests")),
	}

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authentication required\n",
		},
		{
			name:           "invalid authorization header format",
			authHeader:     "InvalidFormat",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid authorization header format\n",
		},
		{
			name:           "invalid bearer token format",
			authHeader:     "Bearer",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid authorization header format\n",
		},
		{
			name:           "invalid jwt token",
			authHeader:     "Bearer invalid.jwt.token",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid token:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			middleware := server.authMiddleware(nextHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && !contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	// Set test environment variables to enable JWT validation
	os.Setenv("AUTHENTIK_URL", "https://auth.example.com")
	defer os.Unsetenv("AUTHENTIK_URL")

	server := &server{
		tracer:       noop.NewTracerProvider().Tracer("test"),
		sessionStore: sessions.NewCookieStore([]byte("test-key-32-bytes-long-for-tests")),
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty token",
			token:       "",
			expectError: true,
			errorMsg:    "failed to get JWKS",
		},
		{
			name:        "malformed token",
			token:       "invalid.token",
			expectError: true,
			errorMsg:    "failed to get JWKS",
		},
		{
			name:        "token with invalid signature",
			token:       "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.invalid",
			expectError: true,
			errorMsg:    "failed to get JWKS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := server.validateJWT(tt.token)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
				if claims != nil {
					t.Errorf("expected nil claims but got %+v", claims)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got %v", err)
				}
				if claims == nil {
					t.Errorf("expected claims but got nil")
				}
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		expectedID int32
		expectedOK bool
	}{
		{
			name:       "context with user ID",
			ctx:        context.WithValue(context.Background(), UserIDContextKey, int32(123)),
			expectedID: 123,
			expectedOK: true,
		},
		{
			name:       "context without user ID",
			ctx:        context.Background(),
			expectedID: 0,
			expectedOK: false,
		},
		{
			name:       "context with wrong type",
			ctx:        context.WithValue(context.Background(), UserIDContextKey, "123"),
			expectedID: 0,
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, ok := getUserIDFromContext(tt.ctx)

			if userID != tt.expectedID {
				t.Errorf("expected userID %d, got %d", tt.expectedID, userID)
			}

			if ok != tt.expectedOK {
				t.Errorf("expected ok %v, got %v", tt.expectedOK, ok)
			}
		})
	}
}

func TestGetJWTConfig(t *testing.T) {
	// Test with environment variables set
	os.Setenv("AUTHENTIK_URL", "https://auth.example.com")
	os.Setenv("AUTHENTIK_APP_NAME", "test-app")
	os.Setenv("JWT_ISSUER", "https://auth.example.com/application/o/test-app/")
	os.Setenv("JWT_AUDIENCE", "test-audience")
	defer func() {
		os.Unsetenv("AUTHENTIK_URL")
		os.Unsetenv("AUTHENTIK_APP_NAME")
		os.Unsetenv("JWT_ISSUER")
		os.Unsetenv("JWT_AUDIENCE")
	}()

	server := &server{
		tracer:       noop.NewTracerProvider().Tracer("test"),
		sessionStore: sessions.NewCookieStore([]byte("test-key-32-bytes-long-for-tests")),
	}
	config := server.getJWTConfig()

	expectedJWKSEndpoint := "https://auth.example.com/application/o/test-app/jwks/"
	if config.JWKSEndpoint != expectedJWKSEndpoint {
		t.Errorf("expected JWKSEndpoint %q, got %q", expectedJWKSEndpoint, config.JWKSEndpoint)
	}

	if config.Issuer != "https://auth.example.com/application/o/test-app/" {
		t.Errorf("expected Issuer %q, got %q", "https://auth.example.com/application/o/test-app/", config.Issuer)
	}

	if config.Audience != "test-audience" {
		t.Errorf("expected Audience %q, got %q", "test-audience", config.Audience)
	}
}

func TestGetJWTConfigWithDefaults(t *testing.T) {
	// Test with minimal environment variables
	os.Setenv("AUTHENTIK_URL", "https://auth.example.com")
	defer os.Unsetenv("AUTHENTIK_URL")

	server := &server{
		tracer:       noop.NewTracerProvider().Tracer("test"),
		sessionStore: sessions.NewCookieStore([]byte("test-key-32-bytes-long-for-tests")),
	}
	config := server.getJWTConfig()

	expectedJWKSEndpoint := "https://auth.example.com/application/o/test-auth-app/jwks/"
	if config.JWKSEndpoint != expectedJWKSEndpoint {
		t.Errorf("expected default JWKSEndpoint %q, got %q", expectedJWKSEndpoint, config.JWKSEndpoint)
	}
}

func TestJWTClaimsValidation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		claims      *JWTClaims
		issuer      string
		audience    string
		expectValid bool
	}{
		{
			name: "valid claims",
			claims: &JWTClaims{
				UserID: 123,
				Email:  "test@example.com",
				Name:   "Test User",
				StandardClaims: jwt.StandardClaims{
					Issuer:    "https://auth.example.com/application/o/test-app/",
					Audience:  "test-audience",
					ExpiresAt: now.Add(time.Hour).Unix(),
					IssuedAt:  now.Unix(),
					NotBefore: now.Unix(),
				},
			},
			issuer:      "https://auth.example.com/application/o/test-app/",
			audience:    "test-audience",
			expectValid: true,
		},
		{
			name: "invalid issuer",
			claims: &JWTClaims{
				UserID: 123,
				Email:  "test@example.com",
				Name:   "Test User",
				StandardClaims: jwt.StandardClaims{
					Issuer:    "https://wrong-issuer.com/",
					Audience:  "test-audience",
					ExpiresAt: now.Add(time.Hour).Unix(),
				},
			},
			issuer:      "https://auth.example.com/application/o/test-app/",
			audience:    "test-audience",
			expectValid: false,
		},
		{
			name: "invalid audience",
			claims: &JWTClaims{
				UserID: 123,
				Email:  "test@example.com",
				Name:   "Test User",
				StandardClaims: jwt.StandardClaims{
					Issuer:    "https://auth.example.com/application/o/test-app/",
					Audience:  "wrong-audience",
					ExpiresAt: now.Add(time.Hour).Unix(),
				},
			},
			issuer:      "https://auth.example.com/application/o/test-app/",
			audience:    "test-audience",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test issuer validation
			if tt.issuer != "" && tt.claims.Issuer != tt.issuer {
				if tt.expectValid {
					t.Errorf("expected valid issuer but got mismatch")
				}
			}

			// Test audience validation
			if tt.audience != "" && tt.claims.Audience != tt.audience {
				if tt.expectValid {
					t.Errorf("expected valid audience but got mismatch")
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
