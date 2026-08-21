package audit

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// RemoteObserver is an audit sink that sends each event as a POST request
// to a remote HTTP server.
type RemoteObserver struct {
	url    string
	client *http.Client
}

// NewRemoteObserver creates an observer that sends events to url.
func NewRemoteObserver(url string) *RemoteObserver {
	return &RemoteObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetID returns the observer's identifier based on the target URL.
func (r *RemoteObserver) GetID() string {
	return "url:" + r.url
}

// Update serializes event to JSON and sends it as a POST request to url.
// Send errors are only logged — Notify does not propagate them.
func (r *RemoteObserver) Update(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Println(err)
		return
	}

	resp, err := r.client.Post(r.url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
}
