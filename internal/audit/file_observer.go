package audit

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type FileObserver struct {
	file *os.File
	mu   sync.Mutex
}

func NewFileObserver(path string) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &FileObserver{file: file}, nil
}

func (f *FileObserver) GetID() string {
	return "file:" + f.file.Name()
}

func (f *FileObserver) Update(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Println(err)
		return
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := f.file.Write(data); err != nil {
		log.Println(err)
	}
}

func (f *FileObserver) Close() error {
	return f.file.Close()
}
