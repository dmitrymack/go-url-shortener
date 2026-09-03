// Package audit implements request auditing for the URL shortener service
// using the Observer pattern: a publisher (Publisher) broadcasts events
// (Event) to all registered observers (Observer) — e.g. FileObserver or
// RemoteObserver.
package audit

//go:generate go run github.com/dmitrymack/go-url-shortener.git/cmd/reset github.com/dmitrymack/go-url-shortener.git/...

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Possible values of the Event.Action field.
const (
	// ActionShorten is emitted when a short link is created.
	ActionShorten = "shorten"
	// ActionFollow is emitted when a short link is followed.
	ActionFollow = "follow"
)

// Event is a single audit event.
//
// generate:reset
type Event struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`
	UserID    string `json:"user_id,omitempty"`
	URL       string `json:"url"`
}

// NewEvent creates an Event with the current timestamp.
func NewEvent(action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}

// Observer is an audit event sink: a file, a remote server, etc. GetID
// returns the observer's unique identifier within a Publisher.
type Observer interface {
	Update(event Event)
	GetID() string
}

// Publisher broadcasts audit events to all its subscribers.
type Publisher interface {
	Register(o Observer)
	Deregister(o Observer)
	Notify(event Event)
}

// eventBufferSize is the number of pending events buffered per observer.
// Once an observer's buffer is full, Notify drops further events for it
// instead of waiting for the observer to catch up.
const eventBufferSize = 100

// observerHandle pairs a registered Observer with the channel and the
// dedicated goroutine that deliver events to it.
type observerHandle struct {
	observer Observer
	events   chan Event
}

// Log is the default thread-safe Publisher implementation. Each registered
// observer gets its own buffered channel and delivery goroutine, so a slow
// or stuck observer can only fall behind — it cannot block Notify or the
// other observers.
type Log struct {
	observers map[string]*observerHandle
	mu        sync.RWMutex
	logger    *zap.SugaredLogger
}

// NewLog creates an empty Log with no subscribers. logger is used to report
// events dropped by Notify.
func NewLog(logger *zap.Logger) *Log {
	return &Log{
		observers: make(map[string]*observerHandle),
		logger:    logger.Sugar(),
	}
}

// Register adds o to the list of subscribers and starts the goroutine that
// delivers events to it.
func (l *Log) Register(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	h := &observerHandle{observer: o, events: make(chan Event, eventBufferSize)}
	l.observers[o.GetID()] = h

	go func() {
		for event := range h.events {
			o.Update(event)
		}
	}()
}

// Deregister removes o from the list of subscribers and stops its delivery
// goroutine.
func (l *Log) Deregister(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if h, ok := l.observers[o.GetID()]; ok {
		delete(l.observers, o.GetID())
		close(h.events)
	}
}

// Notify broadcasts event to all registered observers. The send to each
// observer's channel is non-blocking: if an observer's buffer is full, the
// event is dropped for that observer and Notify moves on. This keeps a slow
// observer from adding backpressure to the request path that calls Notify.
func (l *Log) Notify(event Event) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for id, h := range l.observers {
		select {
		case h.events <- event:
		default:
			l.logger.Warnln("audit: observer is falling behind, dropping event", "id", id)
		}
	}
}
