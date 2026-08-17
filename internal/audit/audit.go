package audit

import (
	"sync"
	"time"
)

const (
	ActionShorten = "shorten"
	ActionFollow  = "follow"
)

type Event struct {
	Timestamp int64  `json:"ts"`
	Action    string `json:"action"`
	UserID    string `json:"user_id,omitempty"`
	URL       string `json:"url"`
}

func NewEvent(action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}

type Observer interface {
	Update(event Event)
	GetID() string
}

type Publisher interface {
	Register(o Observer)
	Deregister(o Observer)
	Notify(event Event)
}

type Log struct {
	observers map[string]Observer
	mu        sync.RWMutex
}

func NewLog() *Log {
	return &Log{observers: make(map[string]Observer)}
}

func (l *Log) Register(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.observers[o.GetID()] = o
}

func (l *Log) Deregister(o Observer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.observers, o.GetID())
}

func (l *Log) Notify(event Event) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, o := range l.observers {
		go o.Update(event)
	}
}
