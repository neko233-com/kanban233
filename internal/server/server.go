package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233/kanban233/internal/api"
	"github.com/neko233/kanban233/internal/auth"
	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/db"
)

type Server struct {
	cfg    *config.Config
	store  *db.Store
	auth   *auth.Service
	http   *http.Server
}

func New(cfg *config.Config) (*Server, error) {
	store, err := db.Open(cfg.Database, cfg.Auth)
	if err != nil {
		return nil, err
	}

	authSvc := auth.NewService(store, cfg.Auth)
	handler := api.NewHandler(store, authSvc, cfg)

	mux := http.NewServeMux()
	handler.Register(mux)
	mux.Handle("/", staticHandler(cfg.Server.StaticDir))

	return &Server{
		cfg:   cfg,
		store: store,
		auth:  authSvc,
		http: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: corsMiddleware(mux),
		},
	}, nil
}

func (s *Server) ListenAndServe() error {
	log.Printf("kanban233 listening on %s", s.cfg.Server.Addr)
	return s.http.ListenAndServe()
}

func (s *Server) Close() error {
	return s.store.Close()
}

func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := r.URL.Path
		if path == "/" || path == "" {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}
		filePath := filepath.Join(dir, filepath.Clean("/"+path))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Agent-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ConfigPath() string {
	if p := os.Getenv("KANBAN_CONFIG"); p != "" {
		return p
	}
	return "server.yaml"
}

func MustLoadConfig() *config.Config {
	cfg, err := config.Load(ConfigPath())
	if err != nil {
		panic(fmt.Sprintf("load config: %v", err))
	}
	return cfg
}
