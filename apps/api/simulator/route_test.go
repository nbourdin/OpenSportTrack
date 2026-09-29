package simulator

import (
	"strings"
	"testing"
)

func TestReadRoute(t *testing.T) {
	input := `<gpx><trk><trkseg>
<trkpt lat="47" lon="-1"><ele>12</ele><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="48" lon="-2"><time>2026-09-29T08:00:01Z</time></trkpt>
</trkseg></trk></gpx>`
	positions, err := ReadRoute(strings.NewReader(input))
	if err != nil || len(positions) != 2 || positions[0].Altitude != 12 || positions[1].Latitude != 48 {
		t.Fatalf("route=%v error=%v", positions, err)
	}
}
