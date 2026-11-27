// Package httpapi provides the HTTP API for registering print requests.
package httpapi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/SonyPradana/simpus-antrian-terminal/internal/config"
	"github.com/SonyPradana/simpus-antrian-terminal/internal/service"
)

const (
	maxBodyBytes = 1 << 20
	headerAPIKey = "X-API-Key"
)

// Printer is the service contract used by the handlers.
type Printer interface {
	Register(ctx context.Context, kode string, force bool) (*service.PrintResult, error)
	Validate(ctx context.Context, kode string, force bool) (*service.PrintResult, error)
	Status(ctx context.Context, kode string) (*service.StatusResult, error)
}

// Server holds the handler dependencies.
type Server struct {
	svc    Printer
	db     *sql.DB
	token  string
	dryRun bool
}

// New creates a Server. With dryRun enabled, POST /api/print only validates.
func New(svc Printer, db *sql.DB, token string, dryRun bool) *Server {
	return &Server{svc: svc, db: db, token: token, dryRun: dryRun}
}

// Handler returns the mux matching docs/api.md.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/print", s.withAuth(s.handlePrint))
	mux.HandleFunc("GET /api/print/{kode}", s.withAuth(s.handleStatus))
	mux.HandleFunc("GET /healthz", s.handleHealth)
	return mux
}

// NewHTTPServer creates an http.Server with production-friendly timeouts.
func NewHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		supplied := r.Header.Get(headerAPIKey)
		if s.token == "" || subtle.ConstantTimeCompare([]byte(supplied), []byte(s.token)) != 1 {
			writeError(w, &service.Error{
				Status:  http.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "X-API-Key is missing or invalid",
			})
			return
		}
		next(w, r)
	}
}

type printRequest struct {
	Kode  string `json:"kode"`
	Force bool   `json:"force,omitempty"`
}

func (s *Server) handlePrint(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req printRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, apiRequestError("invalid JSON body: "+err.Error()))
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, apiRequestError("body must contain a single JSON object"))
		return
	}

	var (
		result *service.PrintResult
		err    error
	)
	if s.dryRun {
		result, err = s.svc.Validate(r.Context(), req.Kode, req.Force)
	} else {
		result, err = s.svc.Register(r.Context(), req.Kode, req.Force)
	}
	if err != nil {
		writeError(w, err)
		return
	}

	log.Printf("print request: kode=%s force=%t remote=%s result=registered",
		result.Kode, req.Force, r.RemoteAddr)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	kode := r.PathValue("kode")
	result, err := s.svc.Status(r.Context(), kode)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	now := time.Now().Format(time.RFC3339)
	if err := s.db.PingContext(ctx); err != nil {
		log.Printf("healthz: database unavailable: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "degraded",
			"db":     "error",
			"time":   now,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"db":     "ok",
		"time":   now,
	})
}

func apiRequestError(message string) *service.Error {
	return &service.Error{
		Status:  http.StatusBadRequest,
		Code:    "INVALID_REQUEST",
		Message: message,
	}
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *service.Error
	if !errors.As(err, &apiErr) {
		log.Printf("unexpected error: %v", err)
		apiErr = &service.Error{
			Status:  http.StatusInternalServerError,
			Code:    "DATABASE_ERROR",
			Message: "internal server error",
		}
	} else if apiErr.Cause != nil {
		log.Printf("error %s: %s | cause: %v", apiErr.Code, apiErr.Message, apiErr.Cause)
	}

	writeJSON(w, apiErr.Status, errorBody{
		Error: errorDetail{Code: apiErr.Code, Message: apiErr.Message},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}
