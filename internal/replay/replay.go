package replay

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"opensporttrack/internal/gpx"
	"opensporttrack/internal/tracking"
)

// Replay streams GPX points and preserves the intervals between their timestamps.
func Replay(ctx context.Context, input io.Reader, speed float64, send func(context.Context, tracking.Sample) error) (int, error) {
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0.01 || speed > 1e9 {
		return 0, fmt.Errorf("speed must be between 0.01 and 1e9")
	}
	reader := gpx.NewReader(input)
	if !reader.Next() {
		if err := reader.Err(); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("GPX contains no track points")
	}
	current := reader.Point()
	count := 0
	nextSendAt := time.Now()
	for {
		hasNext := reader.Next()
		if !hasNext && reader.Err() != nil {
			return count, reader.Err()
		}
		var next gpx.Point
		var delay time.Duration
		if hasNext {
			next = reader.Point()
			interval := next.Time.Sub(current.Time)
			if interval < 0 {
				return count, fmt.Errorf("GPX timestamps are out of order")
			}
			delay = time.Duration(float64(interval) / speed)
		}
		if err := ctx.Err(); err != nil {
			return count, err
		}
		sample := tracking.Sample{Timestamp: current.Time, Position: &tracking.Position{
			Latitude: current.Latitude, Longitude: current.Longitude, Altitude: current.Altitude,
		}}
		// Only advertise intervals the viewer can interpolate with its millisecond hint.
		if hasNext && delay >= time.Millisecond && delay <= 24*time.Hour {
			sample.Next = &tracking.NextPoint{Timestamp: next.Time,
				Position: tracking.Position{Latitude: next.Latitude, Longitude: next.Longitude, Altitude: next.Altitude},
				AfterMS:  delay.Milliseconds()}
		}
		if err := send(ctx, sample); err != nil {
			return count, fmt.Errorf("send GPX point %d: %w", count+1, err)
		}
		count++
		if !hasNext {
			return count, nil
		}
		// Use cumulative deadlines so HTTP latency does not slow every GPX interval.
		nextSendAt = nextSendAt.Add(delay)
		remaining := time.Until(nextSendAt)
		if remaining <= 0 {
			current = next
			continue
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return count, ctx.Err()
		case <-timer.C:
		}
		current = next
	}
}
