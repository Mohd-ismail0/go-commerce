package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RouteRegistrar interface {
	RegisterRoutes(r chi.Router)
}

type Server struct {
	port string
	mux  *chi.Mux
}

func New(port string, middlewares []func(http.Handler) http.Handler, registrars ...RouteRegistrar) *Server {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	for _, m := range middlewares {
		r.Use(m)
	}
	for _, reg := range registrars {
		reg.RegisterRoutes(r)
	}
	return &Server{port: port, mux: r}
}

func (s *Server) Run() error {
	return http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.mux)
}
