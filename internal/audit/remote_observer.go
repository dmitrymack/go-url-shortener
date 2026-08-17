package audit

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type RemoteObserver struct {
	url    string
	client *http.Client
}

func NewRemoteObserver(url string) *RemoteObserver {
	return &RemoteObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *RemoteObserver) GetID() string {
	return "url:" + r.url
}

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
