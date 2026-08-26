package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// RemoteObserver is an audit sink that sends each event as a POST request
// to a remote HTTP server.
type RemoteObserver struct {
	url    string
	client *http.Client
	logger *zap.SugaredLogger
}

// NewRemoteObserver creates an observer that sends events to url. logger is
// used to report request errors, since Update itself cannot return them.
func NewRemoteObserver(url string, logger *zap.Logger) *RemoteObserver {
	return &RemoteObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger.Sugar(),
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
		r.logger.Errorln("audit: failed to marshal event", "error", err)
		return
	}

	resp, err := r.client.Post(r.url, "application/json", bytes.NewReader(data))
	if err != nil {
		r.logger.Errorln("audit: failed to send event", "error", err)
		return
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)
}
