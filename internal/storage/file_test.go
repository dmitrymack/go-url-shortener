package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorage_SetAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	key, err := f.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)
	assert.Equal(t, "abc123", key)

	value, err := f.Get("abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)
}

func TestFileStorage_Get_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Get("unknown")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileStorage_Set_DuplicateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	_, err = f.Set(context.Background(), "abc123", "https://example.org", "user1")
	assert.ErrorIs(t, err, ErrDuplicateKey)
}

func TestFileStorage_PersistsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)

	_, err = f.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)
	_, err = f.Set(context.Background(), "def456", "https://example.org", "user1")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	restarted, err := NewFileStorage(path)
	require.NoError(t, err)
	defer restarted.Close()

	value, err := restarted.Get("abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)

	value, err = restarted.Get("def456")
	require.NoError(t, err)
	assert.Equal(t, "https://example.org", value)
}

func TestFileStorage_SetBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	items := []URLRecord{
		{ID: "abc123", OriginURL: "https://example.com"},
		{ID: "def456", OriginURL: "https://example.org"},
	}

	err = f.SetBatch(context.Background(), items, "user1")
	require.NoError(t, err)

	for _, item := range items {
		value, err := f.Get(item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.OriginURL, value)
	}
}

func TestFileStorage_GetUrlsByUser_NotImplemented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.GetUrlsByUser("user1")
	assert.Error(t, err)
}

func TestFileStorage_SetDeletedBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.jsonl")

	f, err := NewFileStorage(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	err = f.SetDeletedBatch(context.Background(), []string{"abc123"}, "user1")
	require.NoError(t, err)

	_, err = f.Get("abc123")
	assert.ErrorIs(t, err, ErrNotFound)
}
