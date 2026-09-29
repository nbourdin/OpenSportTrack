package replay

import (
	"fmt"
	"io"

	"opensporttrack/internal/gpx"
	"opensporttrack/internal/tracking"
)

// ReadRoute extracts the complete baseline trace before timed replay starts.
// It uses the same streaming GPX reader and caps the viewer route size.
func ReadRoute(input io.Reader) ([]tracking.Position, error) {
	reader := gpx.NewReader(input)
	positions := make([]tracking.Position, 0, 512)
	for reader.Next() {
		if len(positions) == 50000 {
			return nil, fmt.Errorf("GPX route exceeds 50000 points")
		}
		point := reader.Point()
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
