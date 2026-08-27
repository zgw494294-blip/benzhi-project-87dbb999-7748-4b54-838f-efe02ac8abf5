package httpapi

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Server struct {
	http            *http.Server
	shutdownTimeout time.Duration
}

func NewServer(handler http.Handler, shutdownTimeout time.Duration) *Server {
	return &Server{http: &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}, shutdownTimeout: shutdownTimeout}
}

func (s *Server) Serve(listener net.Listener) error { return s.http.Serve(listener) }

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	return s.http.Shutdown(ctx)
}
