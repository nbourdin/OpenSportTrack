package tracking

import "fmt"

const commandBufferSize = 64
const subscriberBufferSize = 64

type activityRuntime struct {
	activity Activity // Immutable after creation; safe to read through the registry.
	commands chan any
	stop     chan struct{}
	done     chan struct{}
}

type activityState struct {
	samples     []Sample
	route       []Position
	subscribers map[chan Message]struct{}
}

type addSampleCommand struct {
	sample Sample
	reply  chan error
}

type setRouteCommand struct {
	positions []Position
	reply     chan error
}

type subscribeCommand struct {
	reply chan subscription
}

type subscription struct {
	snapshot Message
	messages chan Message
}

type unsubscribeCommand struct {
	messages chan Message
	reply    chan struct{}
}

func newActivityRuntime(activity Activity) *activityRuntime {
	// The bounded queue applies backpressure to callers rather than dropping telemetry.
	return &activityRuntime{
		activity: activity,
		commands: make(chan any, commandBufferSize),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (r *activityRuntime) addSample(sample Sample) error {
	reply := make(chan error, 1)
	r.commands <- addSampleCommand{sample: sample, reply: reply}
	return <-reply
}

func (r *activityRuntime) setRoute(positions []Position) error {
	reply := make(chan error, 1)
	r.commands <- setRouteCommand{positions: positions, reply: reply}
	return <-reply
}

func (r *activityRuntime) subscribe() subscription {
	reply := make(chan subscription, 1)
	r.commands <- subscribeCommand{reply: reply}
	return <-reply
}

func (r *activityRuntime) unsubscribe(messages chan Message) {
	reply := make(chan struct{}, 1)
	select {
	case r.commands <- unsubscribeCommand{messages: messages, reply: reply}:
	case <-r.done:
		return
	}
	// Shutdown may close the channel before a queued unsubscribe is processed.
	select {
	case <-reply:
	case <-r.done:
	}
}

// run is the only goroutine that reads or mutates an activity's state.
func (r *activityRuntime) run() {
	state := activityState{subscribers: make(map[chan Message]struct{})}
	defer close(r.done)
	for {
		// Prefer shutdown even if unsubscribe commands remain queued.
		select {
		case <-r.stop:
			state.closeSubscribers()
			return
		default:
		}
		select {
		case command := <-r.commands:
			switch command := command.(type) {
			case addSampleCommand:
				command.reply <- state.addSample(r.activity.ID, command.sample)
			case setRouteCommand:
				state.route = command.positions
				state.broadcast(Message{Version: 1, Type: "route", ActivityID: r.activity.ID, Route: state.route})
				command.reply <- nil
			case subscribeCommand:
				messages := make(chan Message, subscriberBufferSize)
				// Registration and snapshot creation share one serialized command.
				state.subscribers[messages] = struct{}{}
				activity := r.activity
				command.reply <- subscription{messages: messages, snapshot: Message{
					Version: 1, Type: "snapshot", ActivityID: r.activity.ID, Activity: &activity,
					Samples: cloneSamples(state.samples), Route: append([]Position{}, state.route...),
				}}
			case unsubscribeCommand:
				state.removeSubscriber(command.messages)
				command.reply <- struct{}{}
			}
		case <-r.stop:
			state.closeSubscribers()
			return
		}
	}
}

func (s *activityState) addSample(id string, sample Sample) error {
	if n := len(s.samples); n > 0 && sample.Timestamp.Before(s.samples[n-1].Timestamp) {
		return fmt.Errorf("%w: timestamp precedes last sample", ErrInvalidSample)
	}
	s.samples = append(s.samples, sample)
	s.broadcast(Message{Version: 1, Type: "sample", ActivityID: id, Sample: &sample})
	return nil
}

func (s *activityState) broadcast(message Message) {
	for messages := range s.subscribers {
		// Viewers receive their own payloads and cannot mutate runtime state.
		outgoing := cloneEvent(message)
		select {
		case messages <- outgoing:
		default:
			// A full viewer buffer disconnects that viewer without blocking ingestion.
			s.removeSubscriber(messages)
		}
	}
}

func (s *activityState) removeSubscriber(messages chan Message) {
	if _, ok := s.subscribers[messages]; ok {
		delete(s.subscribers, messages)
		close(messages)
	}
}

func (s *activityState) closeSubscribers() {
	for messages := range s.subscribers {
		s.removeSubscriber(messages)
	}
}

func cloneSample(sample Sample) Sample {
	position := *sample.Position
	sample.Position = &position
	if sample.Next != nil {
		next := *sample.Next
		sample.Next = &next
	}
	return sample
}

func cloneSamples(samples []Sample) []Sample {
	result := make([]Sample, len(samples))
	for i, sample := range samples {
		result[i] = cloneSample(sample)
	}
	return result
}

func cloneEvent(message Message) Message {
	if message.Sample != nil {
		sample := cloneSample(*message.Sample)
		message.Sample = &sample
	}
	if message.Route != nil {
		message.Route = append([]Position(nil), message.Route...)
	}
	return message
}
