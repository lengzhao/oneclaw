package statichttp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
	cwd        string // engine working directory; required for multipart file → mediastore
	turnMu     sync.Mutex
	turnSeq    atomic.Uint64
}

// New builds a static HTTP channel connector.
func New(cfg channel.ConnectorConfig) (channel.Connector, error) {
	cwd := ""
	if cfg.Engine != nil {
		cwd = cfg.Engine.CWD
	}
	return &Server{
		listenAddr: resolveListenAddr(cfg.Params),
		staticDir:  resolveStaticDir(cfg.Params),
		cwd:        cwd,
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
		ReadTimeout:       3 * time.Minute, // multipart file uploads
		WriteTimeout:      3 * time.Minute,
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

type chatAttachment struct {
	Name string `json:"name"`
	MIME string `json:"mime,omitempty"`
	Text string `json:"text,omitempty"`
	Path string `json:"path,omitempty"`
}

type chatRequest struct {
	Text        string           `json:"text"`
	Attachments []chatAttachment `json:"attachments,omitempty"`
	Locale      string           `json:"locale,omitempty"`
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

	ct := r.Header.Get("Content-Type")
	ctLower := strings.ToLower(ct)
	var req chatRequest
	if strings.HasPrefix(ctLower, "multipart/form-data") {
		parsed, merr := parseChatMultipart(s.cwd, r)
		if merr != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(chatResponse{Error: merr.Error()})
			return
		}
		req = parsed
	} else {
		if jerr := json.NewDecoder(r.Body).Decode(&req); jerr != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(chatResponse{Error: "invalid JSON body"})
			return
		}
	}
	if strings.TrimSpace(req.Text) == "" && len(req.Attachments) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(chatResponse{Error: "empty text and no attachments"})
		return
	}

	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	n := s.turnSeq.Add(1)
	corrID := fmt.Sprintf("statichttp-%d", n)
	done := make(chan error, 1)
	turnCtx := r.Context()

	atts := make([]routing.Attachment, 0, len(req.Attachments))
	for _, a := range req.Attachments {
		atts = append(atts, routing.Attachment{Name: a.Name, MIME: a.MIME, Text: a.Text, Path: a.Path})
	}
	io.InboundChan <- channel.InboundTurn{
		Ctx:           turnCtx,
		Text:          req.Text,
		Attachments:   atts,
		Locale:        req.Locale,
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
