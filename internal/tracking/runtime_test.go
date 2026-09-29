package tracking

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestActivitiesIngestIndependently(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, err := m.Create("running")
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Create("running")
	if err != nil {
		t.Fatal(err)
	}

	const count = 100
	var callers sync.WaitGroup
	errorsCh := make(chan error, 2)
	for _, id := range []string{a.ID, b.ID} {
		callers.Add(1)
		go func() {
			defer callers.Done()
			for range count {
				if err := m.AddSample(id, testSample()); err != nil {
					errorsCh <- err
					return
				}
			}
		}()
	}
	callers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	for _, id := range []string{a.ID, b.ID} {
		snapshot, _, unsubscribe, err := m.Subscribe(id)
		if err != nil {
			t.Fatal(err)
		}
		unsubscribe()
		if len(snapshot.Samples) != count {
			t.Fatalf("activity %s has %d samples, want %d", id, len(snapshot.Samples), count)
		}
	}
}

func TestConcurrentIngestionSerializesSamples(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")

	const callers, perCaller = 8, 25
	var work sync.WaitGroup
	errorsCh := make(chan error, callers)
	for range callers {
		work.Add(1)
		go func() {
			defer work.Done()
			for range perCaller {
				if err := m.AddSample(a.ID, testSample()); err != nil {
					errorsCh <- err
					return
				}
			}
		}()
	}
	work.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	snapshot, _, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if got, want := len(snapshot.Samples), callers*perCaller; got != want {
		t.Fatalf("snapshot has %d samples, want %d", got, want)
	}
}

func TestSnapshotAndLiveEventsHaveNoGap(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")
	for range 4 {
		if err := m.AddSample(a.ID, testSample()); err != nil {
			t.Fatal(err)
		}
	}

	const later = 32 // Below the viewer buffer, so backpressure cannot obscure the boundary.
	start := make(chan struct{})
	published := make(chan error, 1)
	go func() {
		<-start
		for range later {
			if err := m.AddSample(a.ID, testSample()); err != nil {
				published <- err
				return
			}
		}
		published <- nil
	}()
	close(start)
	snapshot, messages, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	count := len(snapshot.Samples)
	for msg := range messages {
		if msg.Type != "sample" {
			t.Fatalf("unexpected message type %q", msg.Type)
		}
		count++
	}
	if count != 4+later {
		t.Fatalf("snapshot and events contain %d samples, want %d", count, 4+later)
	}
}

func TestSlowViewerDoesNotBlockHealthyViewer(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")
	_, slow, cancelSlow, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelSlow()
	_, healthy, cancelHealthy, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelHealthy()

	for range subscriberBufferSize + 1 {
		if err := m.AddSample(a.ID, testSample()); err != nil {
			t.Fatal(err)
		}
		if msg := <-healthy; msg.Type != "sample" {
			t.Fatalf("healthy viewer received %q", msg.Type)
		}
	}
	count := 0
	for range slow {
		count++
	}
	if count != subscriberBufferSize {
		t.Fatalf("slow viewer received %d buffered samples, want %d", count, subscriberBufferSize)
	}
	if err := m.AddSample(a.ID, testSample()); err != nil {
		t.Fatal(err)
	}
	if msg, ok := <-healthy; !ok || msg.Type != "sample" {
		t.Fatalf("healthy viewer disconnected or received %+v", msg)
	}
}

func TestUnsubscribeDuringIngestion(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")
	_, messages, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}

	var work sync.WaitGroup
	work.Add(2)
	go func() {
		defer work.Done()
		for range 100 {
			if err := m.AddSample(a.ID, testSample()); err != nil {
				t.Errorf("add sample: %v", err)
				return
			}
		}
	}()
	go func() {
		defer work.Done()
		unsubscribe()
		unsubscribe()
	}()
	work.Wait()
	for range messages {
	}
}

func TestCloseStopsRuntimesAndRejectsOperations(t *testing.T) {
	m := NewManager()
	a, _ := m.Create("running")
	b, _ := m.Create("running")
	_, messagesA, unsubscribeA, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, messagesB, unsubscribeB, err := m.Subscribe(b.ID)
	if err != nil {
		t.Fatal(err)
	}

	var callers sync.WaitGroup
	start := make(chan struct{})
	for _, id := range []string{a.ID, b.ID} {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-start
			for range 100 {
				if err := m.AddSample(id, testSample()); err != nil {
					if !errors.Is(err, ErrClosed) {
						t.Errorf("add sample: %v", err)
					}
					return
				}
			}
		}()
	}
	close(start)
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager shutdown did not finish")
	}
	callers.Wait()
	unsubscribeA()
	unsubscribeB()
	for range messagesA {
	}
	for range messagesB {
	}
	for _, runtime := range m.activities {
		select {
		case <-runtime.done:
		default:
			t.Fatal("runtime still running after Close")
		}
	}
	var repeated sync.WaitGroup
	for range 2 {
		repeated.Add(1)
		go func() { defer repeated.Done(); m.Close() }()
	}
	repeated.Wait()
	if _, err := m.Create("bad sport"); !errors.Is(err, ErrClosed) {
		t.Fatalf("Create after close: %v", err)
	}
	if _, err := m.GetActivity(a.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("GetActivity after close: %v", err)
	}
	if err := m.AddSample(a.ID, Sample{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("AddSample after close: %v", err)
	}
	if err := m.SetRoute(a.ID, nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("SetRoute after close: %v", err)
	}
	if _, _, _, err := m.Subscribe(a.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("Subscribe after close: %v", err)
	}
}

func TestInputOwnership(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")
	positions := []Position{{Latitude: 47}}
	if err := m.SetRoute(a.ID, positions); err != nil {
		t.Fatal(err)
	}
	position := &Position{Latitude: 48}
	next := &NextPoint{Timestamp: time.Unix(2, 0), Position: Position{Latitude: 49}, AfterMS: 100}
	if err := m.AddSample(a.ID, Sample{Timestamp: time.Unix(1, 0), Position: position, Next: next}); err != nil {
		t.Fatal(err)
	}
	positions[0].Latitude = 0
	position.Latitude = 0
	next.Position.Latitude = 0
	snapshot, _, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if snapshot.Route[0].Latitude != 47 || snapshot.Samples[0].Position.Latitude != 48 || snapshot.Samples[0].Next.Position.Latitude != 49 {
		t.Fatalf("input mutation changed stored state: %+v", snapshot)
	}
}

func TestSubscriberOutputCannotMutateState(t *testing.T) {
	m := NewManager()
	t.Cleanup(m.Close)
	a, _ := m.Create("running")
	if err := m.AddSample(a.ID, testSample()); err != nil {
		t.Fatal(err)
	}
	if err := m.SetRoute(a.ID, []Position{{Latitude: 47}}); err != nil {
		t.Fatal(err)
	}
	snapshot, messages, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	snapshot.Activity.ID = "changed"
	snapshot.Samples[0].Position.Latitude = 0
	snapshot.Route[0].Latitude = 0

	if err := m.AddSample(a.ID, Sample{Timestamp: time.Unix(2, 0), Position: &Position{Latitude: 48}}); err != nil {
		t.Fatal(err)
	}
	if event := <-messages; event.Sample == nil {
		t.Fatalf("missing sample event: %+v", event)
	} else {
		event.Sample.Position.Latitude = 0
	}
	if err := m.SetRoute(a.ID, []Position{{Latitude: 49}}); err != nil {
		t.Fatal(err)
	}
	if event := <-messages; len(event.Route) != 1 {
		t.Fatalf("missing route event: %+v", event)
	} else {
		event.Route[0].Latitude = 0
	}

	fresh, _, cancelFresh, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelFresh()
	if fresh.Activity.ID != a.ID || fresh.Samples[0].Position.Latitude != 47 ||
		fresh.Samples[1].Position.Latitude != 48 || fresh.Route[0].Latitude != 49 {
		t.Fatalf("subscriber mutation changed runtime state: %+v", fresh)
	}
}

func TestUnsubscribeDuringShutdown(t *testing.T) {
	m := NewManager()
	a, _ := m.Create("running")
	_, messages, unsubscribe, err := m.Subscribe(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var work sync.WaitGroup
	work.Add(2)
	go func() { defer work.Done(); <-start; unsubscribe() }()
	go func() { defer work.Done(); <-start; m.Close() }()
	close(start)
	done := make(chan struct{})
	go func() { work.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("unsubscribe and shutdown did not finish")
	}
	select {
	case _, ok := <-messages:
		if ok {
			t.Fatal("subscriber channel still open")
		}
	default:
		t.Fatal("subscriber channel still open")
	}
}

func BenchmarkAddSample(b *testing.B) {
	m := NewManager()
	b.Cleanup(m.Close)
	a, err := m.Create("running")
	if err != nil {
		b.Fatal(err)
	}
	sample := testSample()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := m.AddSample(a.ID, sample); err != nil {
			b.Fatal(err)
		}
	}
}

func testSample() Sample {
	return Sample{Timestamp: time.Unix(1, 0), Position: &Position{Latitude: 47, Longitude: -1}}
}
