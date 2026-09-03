package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

// FileStorage is a thread-safe short link store that keeps data in memory
// and sequentially appends each new record to a file on disk in JSON Lines
// format. On creation it reads and restores state from an existing file.
type FileStorage struct {
	data map[string]string
	file *os.File
	mu   sync.RWMutex
}

// Event is a single line of the FileStorage backing file: a short/original
// URL pair.
type Event struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// NewFileStorage opens (creating if necessary) the file filename and
// restores the store's state from it.
func NewFileStorage(filename string) (*FileStorage, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	store := &FileStorage{
		data: make(map[string]string),
		file: file,
	}

	scanner := bufio.NewScanner(file)

	for {
		event, err := readEvent(scanner)

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		store.data[event.ShortURL] = event.OriginalURL
	}

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// Get returns the original URL stored under key.
func (f *FileStorage) Get(key string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	value, ok := f.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

// Set stores value under key in memory and appends the corresponding record
// to the end of the file. If the key is already taken, it returns
// ErrDuplicateKey.
func (f *FileStorage) Set(ctx context.Context, key string, value string, userID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.data[key]; ok {
		return key, ErrDuplicateKey
	}

	err := writeEvent(f, &Event{
		ShortURL:    key,
		OriginalURL: value,
	})

	if err != nil {
		return "", err
	}

	f.data[key] = value
	return key, nil
}

// SetBatch sequentially stores all of batchItems.
func (f *FileStorage) SetBatch(ctx context.Context, batchItems []URLRecord, userID string) error {
	for _, item := range batchItems {
		if _, err := f.Set(ctx, item.ID, item.OriginURL, userID); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the storage file.
func (f *FileStorage) Close() error {
	return f.file.Close()
}

// readEvent reads and parses a single JSON line from the storage file.
// Returns io.EOF once there are no more lines.
func readEvent(s *bufio.Scanner) (*Event, error) {
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		return nil, io.EOF
	}

	data := s.Bytes()

	var event Event
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// writeEvent serializes event to JSON and appends it as a new line to the
// end of the storage file.
func writeEvent(f *FileStorage, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.file.Write(data)
	return err
}

// GetUrlsByUser is not implemented for the file store, since it does not
// track link ownership.
func (f *FileStorage) GetUrlsByUser(userID string) ([]URLRecord, error) {
	return nil, errors.New("not implemented")
}

// SetDeletedBatch removes the records for keys from memory. The file on
// disk is not rewritten.
func (f *FileStorage) SetDeletedBatch(ctx context.Context, keys []string, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, key := range keys {
		delete(f.data, key)
	}

	return nil
}
