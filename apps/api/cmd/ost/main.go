package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opensporttrack/apps/api/internal/server"
	"opensporttrack/apps/api/internal/tracking"
	"opensporttrack/apps/api/simulator"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "ost:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ost server [--addr 127.0.0.1:8081] | ost simulator replay FILE [--speed 10] [--server http://localhost:8081] [--web-url http://localhost:5173]")
	}
	switch args[0] {
	case "server":
		flags := flag.NewFlagSet("server", flag.ContinueOnError)
		addr := flags.String("addr", "127.0.0.1:8081", "HTTP listen address")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected server arguments: %v", flags.Args())
		}
		return serve(ctx, *addr)
	case "simulator":
		if len(args) < 3 || args[1] != "replay" {
			return fmt.Errorf("usage: ost simulator replay FILE [--speed 10] [--server URL]")
		}
		file := args[2]
		flags := flag.NewFlagSet("replay", flag.ContinueOnError)
		speed := flags.Float64("speed", 1, "replay speed multiplier")
		serverURL := flags.String("server", "http://localhost:8081", "tracking server URL")
		webURL := flags.String("web-url", "http://localhost:5173", "viewer URL")
		if err := flags.Parse(args[3:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected replay arguments: %v", flags.Args())
		}
		return replay(ctx, file, *speed, *serverURL, *webURL)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
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

func replay(ctx context.Context, path string, speed float64, serverURL, webURL string) error {
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0.01 || speed > 1e9 {
		return fmt.Errorf("speed must be between 0.01 and 1e9")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open GPX: %w", err)
	}
	defer file.Close()
	route, err := simulator.ReadRoute(file)
	if err != nil {
		return fmt.Errorf("read GPX route: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind GPX: %w", err)
	}
	client := simulator.NewClient(serverURL)
	activity, err := client.CreateActivity(ctx)
	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	if err := client.SetRoute(ctx, activity.ID, route); err != nil {
		return fmt.Errorf("upload GPX route: %w", err)
	}
	fmt.Printf("Activity: %s\nViewer: %s/live/%s\n", activity.ID, webURL, activity.ID)
	count, err := simulator.Replay(ctx, file, speed, func(ctx context.Context, sample tracking.Sample) error {
		return client.SendSample(ctx, activity.ID, sample)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Replayed %d points\n", count)
	return nil
}
