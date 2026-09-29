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

// Manager owns the activity registry and coordinates runtime shutdown.
type Manager struct {
	mu         sync.RWMutex
	activities map[string]*activityRuntime
	closed     bool
	operations sync.WaitGroup
	closeDone  chan struct{}
}

func NewManager() *Manager {
	return &Manager{activities: make(map[string]*activityRuntime), closeDone: make(chan struct{})}
}

func (m *Manager) Create(sport string) (Activity, error) {
	if m.isClosed() {
		return Activity{}, ErrClosed
	}
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
	r := newActivityRuntime(a)
	m.activities[a.ID] = r
	go r.run()
	return a, nil
}

func (m *Manager) GetActivity(id string) (Activity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return Activity{}, ErrClosed
	}
	r, ok := m.activities[id]
	if !ok {
		return Activity{}, ErrNotFound
	}
	return r.activity, nil
}

func (m *Manager) AddSample(id string, sample Sample) error {
	if m.isClosed() {
		return ErrClosed
	}
	if sample.Timestamp.IsZero() || sample.Position == nil || !validPosition(*sample.Position) {
		return ErrInvalidSample
	}
	if sample.Next != nil && (!validPosition(sample.Next.Position) || sample.Next.Timestamp.Before(sample.Timestamp) ||
		sample.Next.AfterMS < 1 || sample.Next.AfterMS > 24*60*60*1000) {
		return ErrInvalidSample
	}
	r, err := m.begin(id)
	if err != nil {
		return err
	}
	defer m.operations.Done()
	return r.addSample(cloneSample(sample))
}

func (m *Manager) SetRoute(id string, positions []Position) error {
	if m.isClosed() {
		return ErrClosed
	}
	if len(positions) == 0 || len(positions) > 50000 {
		return ErrInvalidRoute
	}
	for _, position := range positions {
		if !validPosition(position) {
			return ErrInvalidRoute
		}
	}
	r, err := m.begin(id)
	if err != nil {
		return err
	}
	defer m.operations.Done()
	return r.setRoute(append([]Position(nil), positions...))
}

func validPosition(position Position) bool {
	return !math.IsNaN(position.Latitude) && !math.IsNaN(position.Longitude) && !math.IsNaN(position.Altitude) &&
		!math.IsInf(position.Latitude, 0) && !math.IsInf(position.Longitude, 0) && !math.IsInf(position.Altitude, 0) &&
		math.Abs(position.Latitude) <= 90 && math.Abs(position.Longitude) <= 180
}

// Subscribe returns a consistent snapshot followed by subsequent events.
func (m *Manager) Subscribe(id string) (Message, <-chan Message, func(), error) {
	r, err := m.begin(id)
	if err != nil {
		return Message{}, nil, nil, err
	}
	defer m.operations.Done()
	result := r.subscribe()
	var once sync.Once
	cancel := func() { once.Do(func() { r.unsubscribe(result.messages) }) }
	return result.snapshot, result.messages, cancel, nil
}

// begin admits an operation before releasing the registry lock, so Close can
// wait for every accepted command without holding that lock itself.
func (m *Manager) begin(id string) (*activityRuntime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, ErrClosed
	}
	r, ok := m.activities[id]
	if !ok {
		return nil, ErrNotFound
	}
	m.operations.Add(1)
	return r, nil
}

func (m *Manager) isClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		<-m.closeDone
		return
	}
	m.closed = true
	runtimes := make([]*activityRuntime, 0, len(m.activities))
	for _, r := range m.activities {
		runtimes = append(runtimes, r)
	}
	m.mu.Unlock()

	m.operations.Wait()
	for _, r := range runtimes {
		close(r.stop)
	}
	for _, r := range runtimes {
		<-r.done
	}
	close(m.closeDone)
}
