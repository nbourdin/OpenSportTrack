package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opensporttrack/internal/server"
	"opensporttrack/internal/tracking"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "ost-api:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("ost-api", flag.ContinueOnError)
	addr := flags.String("addr", "127.0.0.1:8081", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return serve(ctx, *addr)
}

func serve(ctx context.Context, addr string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	manager := tracking.NewManager()
	defer manager.Close()
	srv := &http.Server{Addr: addr, Handler: server.NewHandler(ctx, manager, logger),
		ReadHeaderTimeout: 5 * time.Second}
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- srv.ListenAndServe() }()
	logger.Info("server listening", "address", addr)
	select {
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	}
}
