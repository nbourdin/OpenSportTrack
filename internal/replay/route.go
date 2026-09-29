package replay

import (
	"fmt"
	"io"
	"time"

	"opensporttrack/internal/gpx"
	"opensporttrack/internal/tracking"
)

// ReadRoute extracts the complete baseline trace before timed replay starts.
// It uses the same streaming GPX reader and caps the viewer route size.
func ReadRoute(input io.Reader) ([]tracking.Position, error) {
	reader := gpx.NewReader(input)
	positions := make([]tracking.Position, 0, 512)
	var previousTime time.Time
	for reader.Next() {
		if len(positions) == 50000 {
			return nil, fmt.Errorf("GPX route exceeds 50000 points")
		}
		point := reader.Point()
		if !previousTime.IsZero() && point.Time.Before(previousTime) {
			return nil, fmt.Errorf("GPX timestamps are out of order at point %d", len(positions)+1)
		}
		previousTime = point.Time
		positions = append(positions, tracking.Position{
			Latitude: point.Latitude, Longitude: point.Longitude, Altitude: point.Altitude,
		})
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, fmt.Errorf("GPX contains no track points")
	}
	return positions, nil
}
