// Package openaistub is a minimal OpenAI-compatible HTTP server for integration tests.
// Point OPENAI_BASE_URL at BaseURL() and set ONCLAW_CHAT_TRANSPORT=non_stream.
package openaistub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Server serves POST /v1/chat/completions with a FIFO queue of raw JSON bodies.
type Server struct {
	mu    sync.Mutex
	queue [][]byte
	// requestBodies holds a copy of each incoming POST body (for e2e assertions). Cleared by ResetRequestBodies.
	requestBodies [][]byte
	srv           *httptest.Server
}

// New starts a stub server; it is closed on t cleanup.
func New(t *testing.T) *Server {
	t.Helper()
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleCompletions)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

// BaseURL returns the value for OPENAI_BASE_URL (must include /v1/ suffix).
func (s *Server) BaseURL() string {
	return s.srv.URL + "/v1/"
}

// ResetRequestBodies clears captured POST bodies (e.g. between manual stub phases).
func (s *Server) ResetRequestBodies() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestBodies = nil
}

// ChatRequestBodies returns a copy of all captured /v1/chat/completions request bodies since start or last reset.
func (s *Server) ChatRequestBodies() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.requestBodies))
	for i, b := range s.requestBodies {
		out[i] = append([]byte(nil), b...)
	}
	return out
}

// Enqueue appends a full chat.completion JSON response body (one per model request).
func (s *Server) Enqueue(body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, body)
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bodyBytes, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestBodies = append(s.requestBodies, append([]byte(nil), bodyBytes...))
	if len(s.queue) == 0 {
		http.Error(w, "stub: empty response queue", http.StatusInternalServerError)
		return
	}
	body := s.queue[0]
	s.queue = s.queue[1:]

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
