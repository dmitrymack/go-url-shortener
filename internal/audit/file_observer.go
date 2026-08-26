package audit

import (
	"encoding/json"
	"os"
	"sync"

	"go.uber.org/zap"
)

// FileObserver is an audit sink that appends each event as a JSON line to
// the end of a file on disk.
type FileObserver struct {
	file   *os.File
	mu     sync.Mutex
	logger *zap.SugaredLogger
}

// NewFileObserver opens (creating if necessary) the file at path for
// appending audit events. logger is used to report write and marshal
// errors, since Update itself cannot return them.
func NewFileObserver(path string, logger *zap.Logger) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &FileObserver{file: file, logger: logger.Sugar()}, nil
}

// GetID returns the observer's identifier based on the file path.
func (f *FileObserver) GetID() string {
	return "file:" + f.file.Name()
}

// Update serializes event to JSON and appends it as a new line to the end
// of the file.
func (f *FileObserver) Update(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		f.logger.Errorln("audit: failed to marshal event", "error", err)
		return
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := f.file.Write(data); err != nil {
		f.logger.Errorln("audit: failed to write event to file", "error", err)
	}
}

// Close closes the observer's file.
func (f *FileObserver) Close() error {
	return f.file.Close()
}
