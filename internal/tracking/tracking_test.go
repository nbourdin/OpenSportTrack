package tracking

import (
	"errors"
	"testing"
	"time"
)

func TestSnapshotAndSubsequentSamples(t *testing.T) {
	m := NewManager()
	activity, err := m.Create("running")
	if err != nil {
		t.Fatal(err)
	}
	first := Sample{Timestamp: time.Now(), Position: &Position{Latitude: 47, Longitude: -1}}
	if err := m.AddSample(activity.ID, first); err != nil {
		t.Fatal(err)
	}
	snapshot, messages, unsubscribe, err := m.Subscribe(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if snapshot.Type != "snapshot" || snapshot.Version != 1 || len(snapshot.Samples) != 1 || snapshot.Samples[0].Position.Latitude != 47 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	second := Sample{Timestamp: first.Timestamp.Add(time.Second), Position: &Position{Latitude: 48, Longitude: -1}}
	if err := m.AddSample(activity.ID, second); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-messages:
		if got.Type != "sample" || got.Sample.Position.Latitude != 48 {
			t.Fatalf("unexpected message: %+v", got)
		}
	default:
		t.Fatal("missing live sample")
	}
}

func TestSlowSubscriberIsDisconnected(t *testing.T) {
	m := NewManager()
	activity, _ := m.Create("running")
	_, messages, unsubscribe, err := m.Subscribe(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	for i := 0; i < 65; i++ {
		err := m.AddSample(activity.ID, Sample{Timestamp: time.Unix(int64(i+1), 0), Position: &Position{Latitude: 47}})
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for range messages {
		count++
	}
	if count != 64 {
		t.Fatalf("received %d buffered samples, want 64", count)
	}
}

func TestRejectsOutOfOrderSample(t *testing.T) {
	m := NewManager()
	activity, _ := m.Create("running")
	sample := Sample{Timestamp: time.Unix(100, 0), Position: &Position{Latitude: 47}}
	if err := m.AddSample(activity.ID, sample); err != nil {
		t.Fatal(err)
	}
	sample.Timestamp = sample.Timestamp.Add(-time.Second)
	if err := m.AddSample(activity.ID, sample); !errors.Is(err, ErrInvalidSample) {
		t.Fatalf("expected ErrInvalidSample, got %v", err)
	}
}
