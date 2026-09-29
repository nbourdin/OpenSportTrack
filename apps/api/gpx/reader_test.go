package gpx

import (
	"strings"
	"testing"
)

func TestReaderStreamsTrackPoints(t *testing.T) {
	input := `<gpx><metadata><name>Test</name></metadata><trk><trkseg>
<trkpt lat="47.21808" lon="-1.55199"><ele>22</ele><time>2026-09-29T08:00:00Z</time></trkpt>
<trkpt lat="47.21796" lon="-1.55286"><time>2026-09-29T08:00:10Z</time></trkpt>
</trkseg></trk></gpx>`
	r := NewReader(strings.NewReader(input))
	if !r.Next() || r.Point().Latitude != 47.21808 || r.Point().Altitude != 22 {
		t.Fatalf("unexpected first point: %+v, error: %v", r.Point(), r.Err())
	}
	if !r.Next() || r.Point().Longitude != -1.55286 {
		t.Fatalf("unexpected second point: %+v, error: %v", r.Point(), r.Err())
	}
	if r.Next() || r.Err() != nil {
		t.Fatalf("expected clean EOF, got %v", r.Err())
	}
}

func TestReaderRejectsMissingCoordinates(t *testing.T) {
	r := NewReader(strings.NewReader(`<gpx><trkpt lon="1"><time>2026-09-29T08:00:00Z</time></trkpt></gpx>`))
	if r.Next() || r.Err() == nil {
		t.Fatal("expected error for missing latitude")
	}
}
