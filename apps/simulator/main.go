package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"syscall"

	"opensporttrack/internal/replay"
	"opensporttrack/internal/tracking"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "ost-simulator:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 2 || args[0] != "replay" {
		return fmt.Errorf("usage: ost-simulator replay FILE [--speed 300] [--server http://localhost:8081] [--web-url http://localhost:5173]")
	}
	file := args[1]
	flags := flag.NewFlagSet("replay", flag.ContinueOnError)
	speed := flags.Float64("speed", 1, "replay speed multiplier")
	serverURL := flags.String("server", "http://localhost:8081", "tracking server URL")
	webURL := flags.String("web-url", "http://localhost:5173", "viewer URL")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected replay arguments: %v", flags.Args())
	}
	return replayGPX(ctx, file, *speed, *serverURL, *webURL)
}

func replayGPX(ctx context.Context, path string, speed float64, serverURL, webURL string) error {
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0.01 || speed > 1e9 {
		return fmt.Errorf("speed must be between 0.01 and 1e9")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open GPX: %w", err)
	}
	defer file.Close()
	route, err := replay.ReadRoute(file)
	if err != nil {
		return fmt.Errorf("read GPX route: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind GPX: %w", err)
	}
	client := replay.NewClient(serverURL)
	activity, err := client.CreateActivity(ctx)
	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	if err := client.SetRoute(ctx, activity.ID, route); err != nil {
		return fmt.Errorf("upload GPX route: %w", err)
	}
	fmt.Printf("Activity: %s\nViewer: %s/live/%s\n", activity.ID, webURL, activity.ID)
	count, err := replay.Replay(ctx, file, speed, func(ctx context.Context, sample tracking.Sample) error {
		return client.SendSample(ctx, activity.ID, sample)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Replayed %d points\n", count)
	return nil
}
