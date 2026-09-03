package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// maxSendAttempts is how many times Update tries to deliver a single event
// before giving up. initialBackoff is the delay before the first retry;
// it doubles after each subsequent failed attempt.
const (
	maxSendAttempts = 3
	initialBackoff  = 200 * time.Millisecond
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

// Update serializes event to JSON and sends it as a POST request to url,
// retrying transient failures (network errors, 5xx responses) up to
// maxSendAttempts times with exponential backoff. Send errors are only
// logged — Notify does not propagate them.
func (r *RemoteObserver) Update(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		r.logger.Errorln("audit: failed to marshal event", "error", err)
		return
	}

	backoff := initialBackoff
	for attempt := 1; attempt <= maxSendAttempts; attempt++ {
		retry, err := r.send(data)
		if err == nil {
			return
		}
		if !retry || attempt == maxSendAttempts {
			r.logger.Errorln("audit: failed to send event", "attempt", attempt, "error", err)
			return
		}
		r.logger.Warnln("audit: retrying event delivery", "attempt", attempt, "error", err)
		time.Sleep(backoff)
		backoff *= 2
	}
}

// send performs a single delivery attempt. The retry return reports whether
// the failure is transient and worth retrying — network errors and 5xx
// responses — as opposed to permanent: a 4xx response means the request
// itself is bad, and retrying an identical request won't change that.
func (r *RemoteObserver) send(data []byte) (retry bool, err error) {
	resp, err := r.client.Post(r.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode >= 500:
		return true, fmt.Errorf("server returned status %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		return false, fmt.Errorf("server returned status %d", resp.StatusCode)
	default:
		return false, nil
	}
}
