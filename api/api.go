package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"

	"github.com/apcichewicz/scratch/database"
	"github.com/apcichewicz/scratch/repositories"
	"github.com/gorilla/sessions"
	"go.opentelemetry.io/otel/trace"
)

type server struct {
	userRepo     repositories.UserRepository
	authRepo     repositories.AuthRepository
	tracer       trace.Tracer
	sessionStore sessions.Store
}

func NewServer(tracer trace.Tracer, queries *database.Queries) *server {
	userRepo := repositories.NewUserRepository(queries)
	authRepo := repositories.NewAuthRepository(queries)

	sessionStore := sessions.NewCookieStore([]byte(getSessionKey()))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   getEnvBool("PRODUCTION", false),
		SameSite: http.SameSiteLaxMode,
	}

	return &server{
		userRepo:     userRepo,
		authRepo:     authRepo,
		tracer:       tracer,
		sessionStore: sessionStore,
	}
}

func getSessionKey() string {
	key := os.Getenv("SESSION_SECRET")
	if key == "" {
		if os.Getenv("ENVIRONMENT") == "development" {
			bytes := make([]byte, 32)
			rand.Read(bytes)
			return base64.StdEncoding.EncodeToString(bytes)
		}
		panic("SESSION_SECRET environment variable is required")
	}
	return key
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	return defaultValue
}

func (s *server) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users", s.getUsers)
	mux.HandleFunc("POST /users", s.createUser)

	mux.HandleFunc("GET /auth/login/{provider}", s.handleLogin)
	mux.HandleFunc("GET /auth/login/{provider}/callback", s.handleCallback)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/user", s.getCurrentUser)

	handler := s.corsMiddleware(mux)

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	server.ListenAndServe()
}

func (s *server) getUsers(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "getUsers")
	defer span.End()
	users, err := s.userRepo.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

func (s *server) createUser(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "createUser")
	defer span.End()
	var user database.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err = s.userRepo.InsertUser(r.Context(), user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
