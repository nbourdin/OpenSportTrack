package replay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"opensporttrack/internal/tracking"
)

func TestReplayPreservesOrder(t *testing.T) {
	input := `<gpx><trk><trkseg>
<trkpt lat="47" lon="-1"><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="48" lon="-2"><time>2026-09-29T08:00:01Z</time></trkpt>
</trkseg></trk></gpx>`
	var points []tracking.Sample
	count, err := Replay(context.Background(), strings.NewReader(input), 1000000, func(_ context.Context, s tracking.Sample) error {
		points = append(points, s)
		return nil
	})
	if err != nil || count != 2 || len(points) != 2 || points[0].Position.Latitude != 47 || points[1].Position.Latitude != 48 {
		t.Fatalf("count=%d points=%v error=%v", count, points, err)
	}
}

func TestReplayAnnouncesNextPoint(t *testing.T) {
	input := `<gpx><trk><trkseg>
<trkpt lat="47" lon="-1"><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="48" lon="-2"><time>2026-09-29T08:00:01Z</time></trkpt>
</trkseg></trk></gpx>`
	var first tracking.Sample
	count, err := Replay(context.Background(), strings.NewReader(input), 1000, func(_ context.Context, s tracking.Sample) error {
		if first.Timestamp.IsZero() {
			first = s
		}
		return nil
	})
	if err != nil || count != 2 || first.Next == nil || first.Next.AfterMS != 1 || first.Next.Position.Latitude != 48 {
		t.Fatalf("count=%d next=%+v error=%v", count, first.Next, err)
	}
}

func TestReplayCancellationDuringWait(t *testing.T) {
	input := `<gpx><trk><trkseg>
<trkpt lat="47" lon="-1"><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="48" lon="-2"><time>2026-09-29T09:00:00Z</time></trkpt>
</trkseg></trk></gpx>`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count, err := Replay(ctx, strings.NewReader(input), 1, func(_ context.Context, _ tracking.Sample) error {
		cancel()
		return nil
	})
	if count != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("count=%d error=%v", count, err)
	}
}

func TestReplayRejectsInvalidSpeed(t *testing.T) {
	_, err := Replay(context.Background(), strings.NewReader(""), 0, func(context.Context, tracking.Sample) error { return nil })
	if err == nil {
		t.Fatal("expected invalid speed error")
	}
}

func TestReplaySendTimeDoesNotAddToGPXInterval(t *testing.T) {
	input := `<gpx><trk><trkseg>
<trkpt lat="47" lon="-1"><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="48" lon="-2"><time>2026-09-29T08:00:00.4Z</time></trkpt>
</trkseg></trk></gpx>`
	var firstSend, secondSend time.Time
	_, err := Replay(context.Background(), strings.NewReader(input), 1, func(_ context.Context, _ tracking.Sample) error {
		if firstSend.IsZero() {
			firstSend = time.Now()
			time.Sleep(200 * time.Millisecond)
		} else {
			secondSend = time.Now()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	interval := secondSend.Sub(firstSend)
	if interval < 350*time.Millisecond || interval > 520*time.Millisecond {
		t.Fatalf("send interval = %v, want approximately 400ms", interval)
	}
}
