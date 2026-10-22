package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

func New(address string, handler http.Handler, shutdownTimeout time.Duration) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		shutdownTimeout: shutdownTimeout,
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
