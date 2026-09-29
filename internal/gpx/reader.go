package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"time"
)

type Point struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Time      time.Time
}

type Reader struct {
	decoder *xml.Decoder
	point   Point
	err     error
}

func NewReader(r io.Reader) *Reader { return &Reader{decoder: xml.NewDecoder(r)} }

func (r *Reader) Next() bool {
	if r.err != nil {
		return false
	}
	for {
		tok, err := r.decoder.Token()
		if err == io.EOF {
			return false
		}
		if err != nil {
			r.err = fmt.Errorf("read GPX: %w", err)
			return false
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "trkpt" {
			continue
		}
		hasLatitude, hasLongitude := false, false
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "lat":
				hasLatitude = true
			case "lon":
				hasLongitude = true
			}
		}
		var raw struct {
			Latitude  float64   `xml:"lat,attr"`
			Longitude float64   `xml:"lon,attr"`
			Altitude  float64   `xml:"ele"`
			Time      time.Time `xml:"time"`
		}
		if err := r.decoder.DecodeElement(&raw, &start); err != nil {
			r.err = fmt.Errorf("decode GPX point: %w", err)
			return false
		}
		if !hasLatitude || !hasLongitude || raw.Time.IsZero() ||
			math.Abs(raw.Latitude) > 90 || math.Abs(raw.Longitude) > 180 ||
			math.IsNaN(raw.Latitude) || math.IsNaN(raw.Longitude) ||
			math.IsNaN(raw.Altitude) || math.IsInf(raw.Altitude, 0) {
			r.err = fmt.Errorf("invalid GPX track point")
			return false
		}
		r.point = Point{Latitude: raw.Latitude, Longitude: raw.Longitude, Altitude: raw.Altitude, Time: raw.Time}
		return true
	}
}

func (r *Reader) Point() Point { return r.point }
func (r *Reader) Err() error   { return r.err }
