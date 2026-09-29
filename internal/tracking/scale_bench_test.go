package tracking

import (
	"fmt"
	goruntime "runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkScaleBurst exercises the Manager API without HTTP or WebSocket overhead.
// It reports how many viewer messages survived the unpaced ingestion burst.
// Equal timestamps keep command scheduling from changing acceptance results.
func BenchmarkScaleBurst(b *testing.B) {
	scenarios := []struct {
		activities int
		producers  int
		viewers    int // Viewers per activity.
	}{
		{1, 8, 0}, {10, 8, 0}, {100, 8, 0}, {1000, 8, 0},
		{10, 32, 0},
		{1, 8, 1}, {10, 8, 1}, {100, 8, 1}, {1000, 8, 1},
		{10, 8, 4}, {10, 32, 4},
	}
	for _, scenario := range scenarios {
		name := fmt.Sprintf("activities=%d/producers=%d/viewers=%d", scenario.activities, scenario.producers, scenario.viewers)
		b.Run(name, func(b *testing.B) {
			benchmarkScaleBurst(b, scenario.activities, scenario.producers, scenario.viewers)
		})
	}
}

func benchmarkScaleBurst(b *testing.B, activities, producers, viewersPerActivity int) {
	goruntime.GC()
	var before goruntime.MemStats
	goruntime.ReadMemStats(&before)
	baselineGoroutines := goruntime.NumGoroutine()

	m := NewManager()
	var readers sync.WaitGroup
	b.Cleanup(func() { m.Close(); readers.Wait() })
	ids := make([]string, activities)
	var delivered atomic.Int64
	for i := range ids {
		activity, err := m.Create("running")
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = activity.ID
		for range viewersPerActivity {
			_, messages, _, err := m.Subscribe(activity.ID)
			if err != nil {
				b.Fatal(err)
			}
			readers.Add(1)
			go func() {
				defer readers.Done()
				for message := range messages {
					if message.Type == "sample" {
						delivered.Add(1)
					}
				}
			}()
		}
	}
	activeGoroutines := goruntime.NumGoroutine() - baselineGoroutines

	sample := Sample{Timestamp: time.Unix(1, 0), Position: &Position{Latitude: 47, Longitude: -1}}
	latencies := make([][]int64, producers)
	errorsCh := make(chan error, producers)
	start := make(chan struct{})
	var work sync.WaitGroup
	for worker := range producers {
		work.Add(1)
		go func() {
			defer work.Done()
			<-start
			var measured []int64
			attempt := 0
			for i := worker; i < b.N; i += producers {
				// Sample 1 in 128 calls to estimate latency without timing every call.
				if attempt%128 == 0 {
					began := time.Now()
					if err := m.AddSample(ids[i%activities], sample); err != nil {
						errorsCh <- err
						return
					}
					measured = append(measured, time.Since(began).Nanoseconds())
				} else if err := m.AddSample(ids[i%activities], sample); err != nil {
					errorsCh <- err
					return
				}
				attempt++
			}
			latencies[worker] = measured
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	close(start)
	work.Wait()
	b.StopTimer()
	close(errorsCh)
	for err := range errorsCh {
		b.Fatal(err)
	}
	if elapsed := b.Elapsed().Seconds(); elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed, "samples/s")
	}
	reportScaleLatency(b, latencies)
	b.ReportMetric(float64(activeGoroutines), "live_goroutines")

	// The runtime retains accepted samples; this is indicative live heap, not allocation rate.
	reportScaleHeap(b, before.HeapAlloc, m)
	m.Close()
	readers.Wait()
	if viewersPerActivity > 0 {
		expected := int64(b.N) * int64(viewersPerActivity)
		b.ReportMetric(100*float64(delivered.Load())/float64(expected), "delivered_pct")
	}
}

// BenchmarkScalePacedFanout waits until every viewer has read each sample.
// This keeps viewers connected and measures sustained end-to-end delivery.
func BenchmarkScalePacedFanout(b *testing.B) {
	for _, scenario := range []struct{ activities, viewers int }{
		{1, 1}, {10, 1}, {100, 1}, {10, 4}, {100, 4},
	} {
		name := fmt.Sprintf("activities=%d/viewers=%d", scenario.activities, scenario.viewers)
		b.Run(name, func(b *testing.B) {
			benchmarkScalePacedFanout(b, scenario.activities, scenario.viewers)
		})
	}
}

type scaleViewer struct {
	ack  chan struct{}
	done chan struct{}
}

func benchmarkScalePacedFanout(b *testing.B, activities, viewersPerActivity int) {
	goruntime.GC()
	var before goruntime.MemStats
	goruntime.ReadMemStats(&before)
	baselineGoroutines := goruntime.NumGoroutine()

	m := NewManager()
	var readers sync.WaitGroup
	b.Cleanup(func() { m.Close(); readers.Wait() })
	ids := make([]string, activities)
	viewers := make([][]scaleViewer, activities)
	var delivered atomic.Int64
	for i := range ids {
		activity, err := m.Create("running")
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = activity.ID
		for range viewersPerActivity {
			_, messages, _, err := m.Subscribe(activity.ID)
			if err != nil {
				b.Fatal(err)
			}
			viewer := scaleViewer{ack: make(chan struct{}, 1), done: make(chan struct{})}
			viewers[i] = append(viewers[i], viewer)
			readers.Add(1)
			go func() {
				defer readers.Done()
				defer close(viewer.done)
				for message := range messages {
					if message.Type == "sample" {
						delivered.Add(1)
						viewer.ack <- struct{}{}
					}
				}
			}()
		}
	}
	activeGoroutines := goruntime.NumGoroutine() - baselineGoroutines

	sample := Sample{Timestamp: time.Unix(1, 0), Position: &Position{Latitude: 47, Longitude: -1}}
	latencies := make([][]int64, activities)
	errorsCh := make(chan error, activities)
	start := make(chan struct{})
	var work sync.WaitGroup
	for activityIndex := range ids {
		work.Add(1)
		go func() {
			defer work.Done()
			<-start
			var measured []int64
			attempt := 0
			for i := activityIndex; i < b.N; i += activities {
				var began time.Time
				if attempt%128 == 0 {
					began = time.Now()
				}
				if err := m.AddSample(ids[activityIndex], sample); err != nil {
					errorsCh <- err
					return
				}
				for _, viewer := range viewers[activityIndex] {
					select {
					case <-viewer.ack:
					case <-viewer.done:
						errorsCh <- fmt.Errorf("viewer disconnected during paced fanout")
						return
					}
				}
				if attempt%128 == 0 {
					measured = append(measured, time.Since(began).Nanoseconds())
				}
				attempt++
			}
			latencies[activityIndex] = measured
		}()
	}

	b.ReportAllocs()
	b.ResetTimer()
	close(start)
	work.Wait()
	b.StopTimer()
	close(errorsCh)
	for err := range errorsCh {
		b.Fatal(err)
	}
	if elapsed := b.Elapsed().Seconds(); elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed, "samples/s")
	}
	reportScaleLatency(b, latencies)
	b.ReportMetric(float64(activeGoroutines), "live_goroutines")
	reportScaleHeap(b, before.HeapAlloc, m)
	m.Close()
	readers.Wait()
	if got, want := delivered.Load(), int64(b.N)*int64(viewersPerActivity); got != want {
		b.Fatalf("delivered %d of %d viewer messages", got, want)
	}
}

func reportScaleLatency(b *testing.B, perWorker [][]int64) {
	var samples []int64
	for _, workerSamples := range perWorker {
		samples = append(samples, workerSamples...)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	if len(samples) > 0 {
		index := (95*len(samples)+99)/100 - 1
		b.ReportMetric(float64(samples[index]), "p95_ns")
	}
}

func reportScaleHeap(b *testing.B, before uint64, m *Manager) {
	goruntime.GC()
	var after goruntime.MemStats
	goruntime.ReadMemStats(&after)
	goruntime.KeepAlive(m)
	if after.HeapAlloc > before {
		b.ReportMetric(float64(after.HeapAlloc-before), "retained_B")
	}
}
