package http

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server exposes the /health and /metrics endpoints.
type Server struct {
	addr string
}

func New(addr string) *Server {
	return &Server{addr: addr}
}

// Start registers routes and listens. It blocks until the server fails.
func (s *Server) Start() {
	log.Fatal(http.ListenAndServe(s.addr, s.handler()))
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", s.healthHandler)
	return mux
}

func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "processor"})
}
