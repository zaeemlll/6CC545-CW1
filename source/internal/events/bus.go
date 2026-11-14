// Package events provides process-local publish/subscribe event delivery.
package events

import (
	"errors"
	"strings"
	"sync"

	"example.edu/6cc545/cw1/internal/config"
)

var ErrClosed = errors.New("event bus closed")

// Kind is the topic. ID is unique within the origin node's event history.
// Only publish metadata; never put file bodies, tokens or keys in an event.
type Event struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Origin string `json:"origin"`
	Name   string `json:"name"`
}

// Delivery reports how many matching subscriber queues accepted or dropped
// an event during one Publish call.
type Delivery struct {
	Delivered int  `json:"delivered"`
	Dropped   int  `json:"dropped"`
	Duplicate bool `json:"duplicate"`
}

type subscriber struct {
	topic string
	ch    chan Event
}

const maxRememberedEvents = 4096

type Bus struct {
	mu          sync.Mutex
	capacity    int
	next        uint64
	closed      bool
	subscribers map[uint64]subscriber
	seen        map[string]struct{}
	seenOrder   []string
}

type Subscription struct {
	C      <-chan Event
	cancel func()
}

// Cancel removes the subscription and closes its receive channel.
// It is safe to call Cancel more than once.
func (s *Subscription) Cancel() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// New creates an event bus with a bounded queue for each subscriber.
func New(capacity int) (*Bus, error) {
	if capacity < 1 || capacity > 1024 {
		return nil, errors.New("capacity must be 1..1024")
	}

	return &Bus{
		capacity:    capacity,
		subscribers: make(map[uint64]subscriber),
		seen:        make(map[string]struct{}),
	}, nil
}

// validTopic checks the topic used for subscription and event filtering.
func validTopic(topic string) bool {
	return topic != "" &&
		len(topic) <= 128 &&
		strings.TrimSpace(topic) == topic &&
		!strings.ContainsRune(topic, 0)
}

// validEvent checks the metadata carried by an event.
func validEvent(e Event) bool {
	if e.ID == "" || len(e.ID) > 128 ||
		strings.TrimSpace(e.ID) != e.ID ||
		strings.ContainsRune(e.ID, 0) {
		return false
	}

	if !validTopic(e.Kind) {
		return false
	}

	if !config.ValidID(e.Origin) {
		return false
	}

	if e.Name == "" || len(e.Name) > 128 ||
		e.Name == "." || e.Name == ".." ||
		strings.ContainsAny(e.Name, `/\:`) ||
		strings.ContainsRune(e.Name, 0) {
		return false
	}

	return true
}

// Subscribe creates an independent bounded queue for the requested topic.
func (b *Bus) Subscribe(topic string) (*Subscription, error) {
	if !validTopic(topic) {
		return nil, errors.New("invalid event topic")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrClosed
	}

	b.next++
	id := b.next

	ch := make(chan Event, b.capacity)
	b.subscribers[id] = subscriber{
		topic: topic,
		ch:    ch,
	}

	return &Subscription{
		C: ch,
		cancel: func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if sub, ok := b.subscribers[id]; ok {
				delete(b.subscribers, id)
				close(sub.ch)
			}
		},
	}, nil
}

// Publish sends an event to every active subscriber matching its topic.
// If a subscriber queue is full, the newest attempted event is dropped.
// Publishing never waits for a subscriber to receive an event.
func (b *Bus) Publish(e Event) (Delivery, error) {
	if !validEvent(e) {
		return Delivery{}, errors.New("invalid event")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return Delivery{}, ErrClosed
	}

	key := e.Origin + "\x00" + e.ID
	if _, exists := b.seen[key]; exists {
		return Delivery{Duplicate: true}, nil
	}

	b.seen[key] = struct{}{}
	b.seenOrder = append(b.seenOrder, key)

	if len(b.seenOrder) > maxRememberedEvents {
		oldest := b.seenOrder[0]
		b.seenOrder = b.seenOrder[1:]
		delete(b.seen, oldest)
	}

	var result Delivery

	for _, sub := range b.subscribers {
		if sub.topic != e.Kind {
			continue
		}

		select {
		case sub.ch <- e:
			result.Delivered++
		default:
			result.Dropped++
		}
	}

	return result, nil
}

// Close shuts down the bus and closes every active subscriber channel.
// It is safe to call Close more than once.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	for id, sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, id)
	}

	return nil
}
