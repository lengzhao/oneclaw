package statichttp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lengzhao/oneclaw/channel"
	"github.com/lengzhao/oneclaw/routing"
)

//go:embed index.html
var indexHTML []byte

// RegistryName is Spec.Key for this connector.
const RegistryName = "statichttp"

// Server serves an embedded static chat page and optional extra files under /static/.
type Server struct {
	listenAddr string
	staticDir  string
	turnMu     sync.Mutex
	turnSeq    atomic.Uint64
}

// New builds a static HTTP channel connector.
func New(cfg channel.ConnectorConfig) (channel.Connector, error) {
	return &Server{
		listenAddr: resolveListenAddr(cfg.Params),
		staticDir:  resolveStaticDir(cfg.Params),
	}, nil
}

func (s *Server) Name() string { return RegistryName }

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context, io channel.IO) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		s.handleChat(w, r, io)
	})
	if dir := s.staticDir; dir != "" {
		// Go 1.22+ ServeMux: unqualified "/static/" matches all methods and conflicts with "GET /".
		fh := http.StripPrefix("/static/", http.FileServer(http.Dir(dir)))
		mux.Handle("GET /static/", fh)
		mux.Handle("HEAD /static/", fh)
	}

	srv := &http.Server{
		Addr:              s.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("statichttp.shutdown", "err", err)
		}
	}()

	slog.Info("statichttp.listen", "addr", s.listenAddr, "static_dir", s.staticDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("statichttp: %w", err)
	}
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

type chatRequest struct {
	Text string `json:"text"`
}

type chatResponse struct {
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, io channel.IO) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Body != nil {
		defer r.Body.Close()
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(chatResponse{Error: "invalid JSON body"})
		return
	}
	if req.Text == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(chatResponse{Error: "empty text"})
		return
	}

	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	n := s.turnSeq.Add(1)
	corrID := fmt.Sprintf("statichttp-%d", n)
	done := make(chan error, 1)
	turnCtx := r.Context()

	io.InboundChan <- channel.InboundTurn{
		Ctx:           turnCtx,
		Text:          req.Text,
		CorrelationID: corrID,
		Done:          done,
	}

	var reply string
	for {
		select {
		case <-turnCtx.Done():
			w.WriteHeader(http.StatusRequestTimeout)
			_ = json.NewEncoder(w).Encode(chatResponse{Error: turnCtx.Err().Error()})
			return
		case rec, ok := <-io.OutboundChan:
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(chatResponse{Error: "outbound closed"})
				return
			}
			switch rec.Kind {
			case routing.KindText:
				if c, _ := rec.Data["content"].(string); c != "" {
					reply += c
				}
			case routing.KindDone:
				okFlag, _ := rec.Data["ok"].(bool)
				if !okFlag {
					msg, _ := rec.Data["error"].(string)
					if msg == "" {
						msg = "turn failed"
					}
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(chatResponse{Error: msg})
					select {
					case <-done:
					case <-turnCtx.Done():
					}
					return
				}
				select {
				case err := <-done:
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(w).Encode(chatResponse{Error: err.Error()})
						return
					}
				case <-turnCtx.Done():
					w.WriteHeader(http.StatusRequestTimeout)
					_ = json.NewEncoder(w).Encode(chatResponse{Error: turnCtx.Err().Error()})
					return
				}
				_ = json.NewEncoder(w).Encode(chatResponse{Reply: reply})
				return
			}
		}
	}
}
