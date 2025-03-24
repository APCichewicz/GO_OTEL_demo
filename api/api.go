package api

import (
	"encoding/json"

	"net/http"

	"github.com/apcichewicz/scratch/database"
	"github.com/apcichewicz/scratch/repositories"
	"go.opentelemetry.io/otel/trace"
)

type server struct {
	userRepo repositories.UserRepository
	tracer   trace.Tracer
}

func NewServer(tracer trace.Tracer, queries *database.Queries) *server {

	userRepo := repositories.NewUserRepository(queries)

	return &server{userRepo: userRepo, tracer: tracer}
}

func (s *server) Start() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users", s.getUsers)
	mux.HandleFunc("POST /users", s.createUser)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
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
