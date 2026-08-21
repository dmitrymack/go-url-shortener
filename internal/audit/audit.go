// Package audit implements request auditing for the URL shortener service
// using the Observer pattern: a publisher (Publisher) broadcasts events
// (Event) to all registered observers (Observer) — e.g. FileObserver or
// RemoteObserver.
package audit

import (
	"sync"
	"time"
)

// Possible values of the Event.Action field.
const (
	// ActionShorten is emitted when a short link is created.
	ActionShorten = "shorten"
	// ActionFollow is emitted when a short link is followed.
	ActionFollow = "follow"
)

// Event is a single audit event.
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

// Log is the default thread-safe Publisher implementation.
type Log struct {
	observers map[string]Observer
	mu        sync.RWMutex
}

// NewLog creates an empty Log with no subscribers.
func NewLog() *Log {
	return &Log{observers: make(map[string]Observer)}
}

// Register adds o to the list of subscribers.
func (l *Log) Register(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.observers[o.GetID()] = o
}

// Deregister removes o from the list of subscribers.
func (l *Log) Deregister(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.observers, o.GetID())
}

// Notify broadcasts event to all registered observers asynchronously (in a
// separate goroutine per subscriber).
func (l *Log) Notify(event Event) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, o := range l.observers {
		go o.Update(event)
	}
}
