package app

import (
	"context"
	"crypto/tls"
	"errors"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/httpapi"
	"example.edu/6cc545/cw1/internal/security"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// Run is a supplied lifecycle example. Explain its ownership and cleanup decisions.
func Run(ctx context.Context, c config.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := CheckStage(c); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ln, err := net.Listen("tcp", c.HTTP)
	if err != nil {
		return fmt.Errorf("listen HTTP %s: %w", c.HTTP, err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { httpapi.ErrorJSON(w, 404, "not_found") })
	mux.HandleFunc("/healthz", httpapi.Health(c.ID))
	mux.HandleFunc("/readyz", httpapi.Readiness(c.ID))

	stop, err := Wire(ctx, c, mux)
	if err != nil {
		// Cancellation during startup is a normal application shutdown.
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer stop()

	handler := http.Handler(mux)
	var publicTLSConfig *tls.Config

	if c.SecurityMode == "required" {
		jwtKey, err := os.ReadFile(c.JWTKeyFile)
		if err != nil {
			return fmt.Errorf("read JWT signing key: %w", err)
		}

		validator := security.TokenValidator{
			Key:      jwtKey,
			Issuer:   c.JWTIssuer,
			Audience: c.JWTAudience,
		}
		handler = security.PublicAPI(validator, mux)

		publicTLSConfig, err = security.ServerTLS(
			c.TLSCertFile,
			c.TLSKeyFile,
			c.CAFile,
			false,
		)
		if err != nil {
			return fmt.Errorf("configure public HTTPS: %w", err)
		}
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: c.Timeout, ReadTimeout: 2 * c.Timeout, WriteTimeout: 2 * c.Timeout, IdleTimeout: 30 * time.Second}
	serveListener := net.Listener(ln)
	protocol := "http"

	if publicTLSConfig != nil {
		serveListener = tls.NewListener(ln, publicTLSConfig)
		protocol = "https"
	}

	done := make(chan error, 1)
	go func() { done <- srv.Serve(serveListener) }()
	log.Printf("node=%s %s=%s ready", c.ID, protocol, ln.Addr())
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// Use a fresh context: the application context is already cancelled.
		shutdownCtx, closeCtx := context.WithTimeout(context.Background(), c.Timeout)
		defer closeCtx()
		shutdownErr := srv.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = srv.Close()
		}
		<-done
		if shutdownErr != nil {
			return fmt.Errorf("HTTP shutdown: %w", shutdownErr)
		}
		return nil
	}
}
