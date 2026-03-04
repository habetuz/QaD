package httpserver

import (
	"io"
	"net/http"
	"strconv"

	"github.com/habetuz/qad/storage"
)

type Server struct {
	storage storage.Storage
}

func NewServer(storage storage.Storage) *Server {
	return &Server{storage: storage}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, key)
	case http.MethodPost:
		s.handlePost(w, r, key)
	case http.MethodDelete:
		s.handleDelete(w, r, key)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleGet returns the value for the given key, or 404 if not found.
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, key string) {
	value := s.storage.Read(key)

	if value == nil {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(value)))
	w.Write(value)
}

// handlePost sets the value for the given key from the request body.
func (s *Server) handlePost(w http.ResponseWriter, r *http.Request, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	go s.writeValue(key, body)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) writeValue(key string, value []byte) {
	s.storage.Write(key, value)
}

// handleDelete clears the value for the given key.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, key string) {
	go s.deleteValue(key)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) deleteValue(key string) {
	s.storage.Delete(key)
}
