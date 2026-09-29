# OpenSportTrack

> Open-source real-time sports telemetry server and protocol.

**Status:** Draft

**Version:** 0.1

**Primary language:** Go

---

## 1. Vision

OpenSportTrack is an open-source project that lets any device or application send sports data in real time to an independent server.

The server receives telemetry, maintains activity state, calculates live metrics, and broadcasts data to connected viewers.

The project aims to support:

- live tracking of an activity;
- experimentation with sports data;
- development of compatible devices;
- self-hosting;
- device simulation without dedicated hardware.

It should also provide a natural use case for Go's strengths:

- concurrency;
- networking;
- streaming;
- efficient I/O;
- low memory usage;
- standalone binaries;
- real-time processing.

---

## 2. MVP

The first MVP should support this scenario:

```text
GPX file
   │
   ▼
Go Simulator
   │
   │ HTTP
   ▼
Go Tracking Server
   │
   ├── Activity state
   ├── Live metrics
   └── Broadcast
           │
           │ WebSocket
           ▼
       Web Viewer
```

Target command:

```bash
docker compose up
```

Then:

```bash
ost simulator replay examples/run.gpx --speed 10
```

The user opens:

```text
http://localhost:8080/live/{activity_id}
```

and watches the activity unfold on a map.

---

## 3. MVP scope

### Included

#### Server

- activity creation;
- telemetry point ingestion;
- support for several concurrent activities;
- real-time metric calculation;
- WebSocket broadcast;
- support for multiple viewers;
- in-memory state;
- graceful shutdown.

#### Simulator

- GPX reading;
- real-time replay;
- time acceleration;
- automatic activity creation;
- sending points to the server.

#### Viewer

- WebSocket connection;
- map;
- current position;
- recorded track;
- distance;
- duration;
- speed;
- pace.

### Not included initially

The MVP does not include:

- authentication;
- user accounts;
- clubs;
- PostgreSQL;
- Redis;
- Kubernetes;
- microservices;
- a mobile application;
- Garmin/Coros/Wahoo integrations;
- FIT;
- activity history;
- advanced analytics.

These can be added when the need arises.

---

## 4. Domain model

### Activity

An `Activity` represents a sports session in progress.

```go
type Activity struct {
    ID        string
    Sport     Sport
    StartedAt time.Time
}
```

Example sports:

```text
running
cycling
trail
walking
skiing
rowing
```

For the MVP, `running` is sufficient.

---

## 5. Telemetry model

A `Sample` is the core of the protocol.

```go
type Sample struct {
    Timestamp time.Time `json:"timestamp"`

    Position *Position `json:"position,omitempty"`

    HeartRate *uint16  `json:"heart_rate,omitempty"`
    Cadence   *uint16  `json:"cadence,omitempty"`
    Power     *uint16  `json:"power,omitempty"`
    Speed     *float64 `json:"speed,omitempty"`
}
```

Position:

```go
type Position struct {
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Altitude  float64 `json:"altitude,omitempty"`
}
```

Metrics are optional so devices with different capabilities can send different data.

```text
Phone
GPS

Running watch
GPS + HR + cadence

Cycling computer
GPS + HR + cadence + power
```

---

## 6. Protocol

The initial protocol should remain simple.

### Create activity

```http
POST /api/v1/activities
```

Request:

```json
{
  "sport": "running"
}
```

Response:

```json
{
  "id": "01JXYZ...",
  "sport": "running",
  "started_at": "2026-09-28T18:00:00Z"
}
```

### Send telemetry

```http
POST /api/v1/activities/{activity_id}/samples
```

Request:

```json
{
  "timestamp": "2026-09-28T18:00:01Z",
  "position": {
    "latitude": 47.2184,
    "longitude": -1.5536,
    "altitude": 42.1
  },
  "heart_rate": 156
}
```

Response:

```http
204 No Content
```

---

## 7. Batch ingestion

Real devices may temporarily lose their connection. The protocol should therefore soon support sending multiple samples at once.

```http
POST /api/v1/activities/{activity_id}/samples/batch
```

```json
{
  "samples": [
    {},
    {},
    {}
  ]
}
```

This enables:

```text
device
   │
   X network unavailable
   │
local buffer
   │
network restored
   ▼
batch upload
```

Batch ingestion can be introduced after the first vertical slice.

---

## 8. Live stream

Viewers use WebSocket.

```text
GET /api/v1/activities/{activity_id}/live
```

Connection:

```text
viewer
   │
   ▼
WebSocket
   │
   ▼
activity stream
```

A possible message:

```json
{
  "type": "sample",
  "activity_id": "01JXYZ",
  "sample": {
    "timestamp": "...",
    "position": {
      "latitude": 47.2184,
      "longitude": -1.5536
    }
  }
}
```

Messages should support versioning.

---

## 9. Server architecture

Conceptual architecture:

```text
HTTP
 │
 ▼
Ingestion Handler
 │
 ▼
Activity Manager
 │
 ▼
Activity Runtime
 │
 ├──────────────► Metrics
 │
 ├──────────────► Storage
 │
 └──────────────► Broadcast
                         │
                         ▼
                    WebSockets
```

`ActivityRuntime` is the central component.

---

## 10. Activity runtime

Each active activity has its own runtime.

Conceptually:

```go
type ActivityRuntime struct {
    activity Activity

    samples chan Sample

    subscribers map[*Subscriber]struct{}
}
```

The runtime has a main loop.

```go
func (r *ActivityRuntime) Run(ctx context.Context) {
    for {
        select {
        case sample := <-r.samples:
            r.process(sample)

        case <-ctx.Done():
            return
        }
    }
}
```

This model naturally exposes:

- goroutines;
- channels;
- cancellation;
- backpressure;
- synchronization;
- lifecycle management.

---

## 11. Concurrency model

Each active activity can have a goroutine.

```text
ActivityManager

 ├── Activity A
 │      └── goroutine
 │
 ├── Activity B
 │      └── goroutine
 │
 └── Activity C
        └── goroutine
```

An incoming sample is sent to the corresponding channel.

```text
HTTP request
     │
     ▼
ActivityManager
     │
     ▼
activity.samples
     │
     ▼
Activity goroutine
```

The HTTP handler should not perform heavy business calculations.

---

## 12. Backpressure

A slow client must never block an activity. Each subscriber therefore has a buffer.

```go
type Subscriber struct {
    Messages chan Message
}
```

For example:

```go
make(chan Message, 64)
```

If the buffer fills up, several strategies could be considered:

```text
drop newest
drop oldest
disconnect slow consumer
```

For the MVP, **disconnect slow consumer** is probably the simplest behavior. It should be measurable and tested.

---
## 13. Metrics

The initial metrics will be calculated incrementally.

### Distance

Distance between two GPS coordinates. Start with **Haversine**.

Total distance:

```text
distance += distance(previousPoint, currentPoint)
```

### Duration

```text
current timestamp - started_at
```

### Speed

If the device does not provide it:

```text
distance delta / time delta
```

### Pace

For running:

```text
pace = duration / distance
```

Example:

```text
4:32 / km
```

---

## 14. Simulator

The simulator is an integral part of the project.

Command:

```bash
ost simulator replay activity.gpx
```

Options:

```bash
--speed 1
--speed 5
--speed 10
--speed 100
```

Example:

```bash
ost simulator replay examples/run.gpx --speed 10
```

A real 50-minute activity will replay in 5 minutes.

---

## 15. GPX reader

The GPX parser should work with `io.Reader`.

Target API:

```go
reader := gpx.NewReader(file)

for reader.Next() {
    point := reader.Point()
}
```

Or possibly:

```go
err := gpx.Read(file, func(point Point) error {
    return simulator.Send(point)
})
```

The goal is to avoid loading the entire file into memory unnecessarily. This provides experience with Go patterns around:

- `io.Reader`;
- streaming;
- iterators;
- error handling.

---

## 16. Replay engine

Replay should respect GPX timestamps.

Example:

```text
GPX

10:00:00 point A
10:00:01 point B
10:00:03 point C
```

At `1x` speed:

```text
A
wait 1s
B
wait 2s
C
```

At `10x` speed:

```text
A
wait 100ms
B
wait 200ms
C
```

Replay should accept a `context.Context` so it can be stopped cleanly.

---

## 17. CLI

Temporary name:

```text
ost
```

Examples:

```bash
ost server
ost simulator replay run.gpx
ost simulator replay run.gpx --speed 20
```

Later:

```bash
ost activity inspect run.fit
ost benchmark ingestion
```

---

## 18. Viewer

The viewer should remain deliberately simple. It should display:

```text
┌──────────────────────────────────────┐
│                                      │
│               MAP                    │
│                                      │
│          ───────●                    │
│                                      │
├──────────────────────────────────────┤
│                                      │
│ Distance       Pace       Duration   │
│                                      │
│ 8.42 km        4:32/km    38:12      │
│                                      │
└──────────────────────────────────────┘
```

Technology to be decided. The viewer is not the core of the project; a lightweight SPA is sufficient.

---

## 19. Repository structure

Proposed initial structure:

```text
opensporttrack/

├── cmd/
│   └── ost/
│
├── internal/
│   ├── activity/
│   ├── ingestion/
│   ├── broadcast/
│   └── metrics/
│
├── pkg/
│   ├── protocol/
│   └── client/
│
├── gpx/
│
├── simulator/
│
├── web/
│
├── examples/
│   └── activities/
│
├── go.mod
├── README.md
└── docker-compose.yml
```

This structure is deliberately provisional. Packages should not be created before a real responsibility emerges.

---

## 20. Dependency policy

The project should prefer the standard library when reasonable. This does not mean **no dependencies**. Every dependency should solve a real problem.

Examples where a dependency may be useful:

- WebSocket;
- CLI;
- structured logging;
- FIT parsing;
- observability.

For HTTP, `net/http` is the initial preference.

---

## 21. Error handling

Errors should be explicit and wrapped with context.

```go
if err != nil {
    return fmt.Errorf("decode GPX point: %w", err)
}
```

No implicit exceptions or `panic` for operational errors.

---

## 22. Context

`context.Context` should be used for operations with a lifecycle.

Examples:

- HTTP requests;
- activity runtime;
- simulator;
- WebSocket;
- server shutdown.

It should not be stored arbitrarily in domain structures.

---

## 23. Graceful shutdown

When the server receives `SIGTERM` or `SIGINT`, it should:

```text
stop accepting connections
        ↓
cancel root context
        ↓
stop activity runtimes
        ↓
close websocket connections
        ↓
wait workers
        ↓
exit
```

This behavior should be tested.

---

## 24. Observability

Use **structured logs** from the start.

Later:

- Prometheus metrics;
- OpenTelemetry;
- pprof.

Useful metrics:

```text
active_activities
connected_viewers
samples_received_total
samples_dropped_total
sample_processing_duration
```

---

## 25. Testing strategy

The project should have many unit tests, especially for:

- GPX parsing;
- distance calculations;
- metrics;
- replay timing;
- activity lifecycle;
- subscriber lifecycle;
- backpressure.

Concurrency tests should regularly run with:

```bash
go test -race ./...
```

---

## 26. Benchmarks

The project should also contain real Go benchmarks.

```go
func BenchmarkActivityIngestion(b *testing.B) {
    // ...
}
```

Future measures:

- samples per second;
- memory per active activity;
- broadcast throughput;
- number of simultaneous viewers.

Avoid arbitrary targets for now. Measure first.

---
## 27. Persistence

### MVP

```text
memory
```

Activities disappear when the server restarts. This is intentional.

### Future

A storage abstraction can be introduced when it is truly needed.

Possibilities:

- PostgreSQL;
- TimescaleDB;
- ClickHouse;
- SQLite.

The choice should follow real usage patterns.

---

## 28. Future device architecture

A future mobile application could act as a device.

```text
Phone GPS
   │
   ▼
Flutter application
   │
   ▼
OpenSportTrack protocol
   │
   ▼
Go server
```

It could:

- start an activity;
- record GPS data;
- buffer samples offline;
- send samples;
- stop an activity.

A phone would allow testing OpenSportTrack with a real GPS signal, without a connected watch.

---

## 29. Future protocol capabilities

The model should be able to grow to support:

- GPS;
- heart rate;
- cadence;
- power;
- temperature;
- altitude;
- speed;
- stroke rate;
- running dynamics.

Only the necessary data should be implemented initially.

---

## 30. Potential hardware integrations

Eventually, adapters could connect:

- Garmin;
- Wahoo;
- Coros;
- Apple Watch;
- Wear OS;
- ANT+;
- Bluetooth LE sensors;
- cycling computers;
- GPS trackers.

These integrations should remain separate from the core server.

---

## 31. Engineering principles

### Keep the core small

The server should remain understandable.

### Prefer explicit code

Avoid premature abstractions.

### Concurrency must solve a real problem

No decorative goroutines. Each goroutine should have:

```text
owner
lifecycle
cancellation strategy
```

### Measure before optimizing

Use benchmarks, pprof, and metrics before introducing complex optimizations.

### Protocol first

Devices should not depend on the server's internal implementation.

```text
Device
   │
   ▼
Protocol
   │
   ▼
Server
```

This allows other server and client implementations to exist.

---

## 32. First implementation milestone

The first vertical slice should be extremely small.

### Milestone 1 — One runner

#### Server

```text
POST activity
POST sample
WebSocket live
```

#### Simulator

```text
GPX reader
replay
HTTP client
```

#### Viewer

```text
WebSocket
map
moving marker
```

No database. No authentication. No heart rate. No multi-device support.

---

## 33. Milestone 2 — Real telemetry engine

Add:

- distance;
- speed;
- pace;
- duration;
- activity state;
- multiple viewers.

Then test:

- slow consumers;
- disconnects;
- invalid samples;
- out-of-order samples.

---

## 34. Milestone 3 — Concurrency

Support several activities:

```text
100 activities
1000 viewers
continuous telemetry
```

Add:

- benchmarks;
- race detector;
- pprof;
- load simulator.

At that point, Go's behavior under load can be observed directly.

---

## 35. Milestone 4 — Persistence

Introduce persistence only at this stage.

Initial needs:

- activity recovery;
- activity history;
- historical tracks.

Storage can be selected based on benchmarks and the real usage model.

---

## 36. Milestone 5 — Real device

Create a minimal mobile application:

```text
START
  ↓
GPS recording
  ↓
live upload
  ↓
STOP
```

The phone then becomes the first real OpenSportTrack device.
