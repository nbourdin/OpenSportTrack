package tracking

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	ErrNotFound      = errors.New("activity not found")
	ErrInvalidSample = errors.New("invalid sample")
	ErrInvalidRoute  = errors.New("invalid route")
	ErrClosed        = errors.New("activity manager closed")
)

type Activity struct {
	ID        string    `json:"id"`
	Sport     string    `json:"sport"`
	StartedAt time.Time `json:"started_at"`
}

type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude,omitempty"`
}

type Sample struct {
	Timestamp time.Time  `json:"timestamp"`
	Position  *Position  `json:"position,omitempty"`
	Next      *NextPoint `json:"next,omitempty"`
}

// NextPoint is an optional display hint for deterministic replays. It lets a
// viewer move toward the next known point while waiting for its telemetry.
type NextPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Position  Position  `json:"position"`
	AfterMS   int64     `json:"after_ms"`
}

type Message struct {
	Version    int        `json:"version"`
	Type       string     `json:"type"`
	ActivityID string     `json:"activity_id"`
	Activity   *Activity  `json:"activity,omitempty"`
	Samples    []Sample   `json:"samples,omitempty"`
	Sample     *Sample    `json:"sample,omitempty"`
	Route      []Position `json:"route,omitempty"`
}

type runtime struct {
	activity    Activity
	samples     []Sample
	route       []Position
	subscribers map[chan Message]struct{}
}

// Manager owns all mutable activity state. A full subscriber buffer disconnects
// that viewer; ingestion never waits for network I/O.
type Manager struct {
	mu         sync.Mutex
	activities map[string]*runtime
	closed     bool
}

func NewManager() *Manager {
	return &Manager{activities: make(map[string]*runtime)}
}

func (m *Manager) Create(sport string) (Activity, error) {
	if sport != "running" {
		return Activity{}, fmt.Errorf("unsupported sport: %q", sport)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Activity{}, fmt.Errorf("generate activity id: %w", err)
	}
	a := Activity{ID: hex.EncodeToString(id[:]), Sport: sport, StartedAt: time.Now().UTC()}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Activity{}, ErrClosed
	}
	m.activities[a.ID] = &runtime{activity: a, subscribers: make(map[chan Message]struct{})}
	return a, nil
}

func (m *Manager) AddSample(id string, sample Sample) error {
	if sample.Timestamp.IsZero() || sample.Position == nil || !validPosition(*sample.Position) {
		return ErrInvalidSample
	}
	if sample.Next != nil && (!validPosition(sample.Next.Position) || sample.Next.Timestamp.Before(sample.Timestamp) ||
		sample.Next.AfterMS < 1 || sample.Next.AfterMS > 24*60*60*1000) {
		return ErrInvalidSample
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	r, ok := m.activities[id]
	if !ok {
		return ErrNotFound
	}
	if n := len(r.samples); n > 0 && sample.Timestamp.Before(r.samples[n-1].Timestamp) {
		return fmt.Errorf("%w: timestamp precedes last sample", ErrInvalidSample)
	}
	r.samples = append(r.samples, sample)
	msg := Message{Version: 1, Type: "sample", ActivityID: id, Sample: &sample}
	for ch := range r.subscribers {
		select {
		case ch <- msg:
		default:
			delete(r.subscribers, ch)
			close(ch)
		}
	}
	return nil
}

func (m *Manager) SetRoute(id string, positions []Position) error {
	if len(positions) == 0 || len(positions) > 50000 {
		return ErrInvalidRoute
	}
	for _, position := range positions {
		if !validPosition(position) {
			return ErrInvalidRoute
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	r, ok := m.activities[id]
	if !ok {
		return ErrNotFound
	}
	r.route = append([]Position(nil), positions...)
	msg := Message{Version: 1, Type: "route", ActivityID: id, Route: r.route}
	for ch := range r.subscribers {
		select {
		case ch <- msg:
		default:
			delete(r.subscribers, ch)
			close(ch)
		}
	}
	return nil
}

func validPosition(position Position) bool {
	return !math.IsNaN(position.Latitude) && !math.IsNaN(position.Longitude) && !math.IsNaN(position.Altitude) &&
		!math.IsInf(position.Latitude, 0) && !math.IsInf(position.Longitude, 0) && !math.IsInf(position.Altitude, 0) &&
		math.Abs(position.Latitude) <= 90 && math.Abs(position.Longitude) <= 180
}

// Subscribe returns a consistent snapshot and all subsequent samples.
func (m *Manager) Subscribe(id string) (Message, <-chan Message, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Message{}, nil, nil, ErrClosed
	}
	r, ok := m.activities[id]
	if !ok {
		return Message{}, nil, nil, ErrNotFound
	}
	ch := make(chan Message, 64)
	r.subscribers[ch] = struct{}{}
	snapshot := Message{Version: 1, Type: "snapshot", ActivityID: id, Activity: &r.activity,
		Samples: append([]Sample{}, r.samples...), Route: append([]Position{}, r.route...)}
	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := r.subscribers[ch]; ok {
			delete(r.subscribers, ch)
			close(ch)
		}
	}
	return snapshot, ch, cancel, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	for _, r := range m.activities {
		for ch := range r.subscribers {
			delete(r.subscribers, ch)
			close(ch)
		}
	}
}
