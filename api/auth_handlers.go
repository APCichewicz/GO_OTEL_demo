package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

type AuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}

type OAuthUserInfo struct {
	ID    string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type SessionData struct {
	Token     string    `json:"token"`
	UserID    int32     `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func init() {
	gob.Register(SessionData{})
}

func (s *server) getAuthConfig(provider string) (*AuthConfig, error) {
	switch provider {
	case "authentik":
		return &AuthConfig{
			ClientID:     os.Getenv("AUTHENTIK_CLIENT_ID"),
			ClientSecret: os.Getenv("AUTHENTIK_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("AUTHENTIK_REDIRECT_URL"),
			AuthURL:      os.Getenv("AUTHENTIK_AUTH_URL"),
			TokenURL:     os.Getenv("AUTHENTIK_TOKEN_URL"),
			UserInfoURL:  os.Getenv("AUTHENTIK_USERINFO_URL"),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *server) getOAuthConfig(provider string) (*oauth2.Config, error) {
	authConfig, err := s.getAuthConfig(provider)
	if err != nil {
		return nil, err
	}

	return &oauth2.Config{
		ClientID:     authConfig.ClientID,
		ClientSecret: authConfig.ClientSecret,
		RedirectURL:  authConfig.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authConfig.AuthURL,
			TokenURL: authConfig.TokenURL,
		},
		Scopes: []string{"openid", "profile", "email"},
	}, nil
}

func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "handleLogin")
	defer span.End()

	provider := r.PathValue("provider")
	if provider == "" {
		http.Error(w, "provider parameter is required", http.StatusBadRequest)
		return
	}

	oauthConfig, err := s.getOAuthConfig(provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state, err := generateState()
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	session, err := s.sessionStore.Get(r, "oauth-session")
	if err != nil {
		http.Error(w, "failed to get session", http.StatusInternalServerError)
		return
	}

	session.Values["state"] = state
	session.Values["provider"] = provider
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	url := oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (s *server) handleCallback(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "handleCallback")
	defer span.End()

	provider := r.PathValue("provider")
	if provider == "" {
		http.Error(w, "provider parameter is required", http.StatusBadRequest)
		return
	}

	session, err := s.sessionStore.Get(r, "oauth-session")
	if err != nil {
		http.Error(w, "failed to get session", http.StatusInternalServerError)
		return
	}

	savedState, ok := session.Values["state"].(string)
	if !ok {
		http.Error(w, "invalid session state", http.StatusBadRequest)
		return
	}

	savedProvider, ok := session.Values["provider"].(string)
	if !ok || savedProvider != provider {
		http.Error(w, "invalid session provider", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if state != savedState {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "code parameter is required", http.StatusBadRequest)
		return
	}

	oauthConfig, err := s.getOAuthConfig(provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	userInfo, err := s.fetchUserInfo(token.AccessToken, provider)
	if err != nil {
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	user, err := s.authRepo.GetUserByOAuth(r.Context(), provider, userInfo.ID)
	if err != nil {
		user, err = s.authRepo.InsertOAuthUser(r.Context(), userInfo.Email, userInfo.Name, provider, userInfo.ID)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
	}

	userSession, err := s.sessionStore.Get(r, "user-session")
	if err != nil {
		http.Error(w, "failed to get user session", http.StatusInternalServerError)
		return
	}

	sessionData := SessionData{
		Token:     token.AccessToken,
		UserID:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Provider:  provider,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	userSession.Values["user"] = sessionData
	err = userSession.Save(r, w)
	if err != nil {
		log.Printf("Failed to save user session: %v", err)
		http.Error(w, "failed to save user session", http.StatusInternalServerError)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	http.Redirect(w, r, frontendURL+"/auth/success", http.StatusTemporaryRedirect)
}

func (s *server) fetchUserInfo(accessToken, provider string) (*OAuthUserInfo, error) {
	authConfig, err := s.getAuthConfig(provider)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", authConfig.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user info: %s", resp.Status)
	}

	var userInfo OAuthUserInfo
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func (s *server) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "getCurrentUser")
	defer span.End()

	session, err := s.sessionStore.Get(r, "user-session")
	if err != nil {
		http.Error(w, "failed to get session", http.StatusInternalServerError)
		return
	}

	userData, ok := session.Values["user"].(SessionData)
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if time.Now().After(userData.ExpiresAt) {
		http.Error(w, "session expired", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userData)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "handleLogout")
	defer span.End()

	session, err := s.sessionStore.Get(r, "user-session")
	if err != nil {
		http.Error(w, "failed to get session", http.StatusInternalServerError)
		return
	}

	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logout successful",
	})
}

func (s *server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}

		w.Header().Set("Access-Control-Allow-Origin", frontendURL)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
