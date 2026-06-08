package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

type FileStorage struct {
	data map[string]string
	file *os.File
}

type Event struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

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

func (f *FileStorage) Get(key string) (string, bool) {
	value, ok := f.data[key]
	return value, ok
}

func (f *FileStorage) Set(ctx context.Context, key string, value string) (string, error) {
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

func (f *FileStorage) SetBatch(ctx context.Context, batchItems []URLRecord) error {
	for _, item := range batchItems {
		if _, err := f.Set(ctx, item.ID, item.OriginURL); err != nil {
			return err
		}
	}

	return nil
}

func (f *FileStorage) Close() error {
	return f.file.Close()
}

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

func writeEvent(f *FileStorage, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.file.Write(data)
	return err
}

func (f *FileStorage) GetUrlsByUser(userID string) ([]URLRecord, error) {
	return nil, errors.New("not implemented")
}
