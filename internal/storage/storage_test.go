package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_SetAndGet(t *testing.T) {
	s := NewStorage()

	key, err := s.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)
	assert.Equal(t, "abc123", key)

	value, err := s.Get("abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value)
}

func TestStorage_Get_NotFound(t *testing.T) {
	s := NewStorage()

	_, err := s.Get("unknown")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStorage_Set_DuplicateKey(t *testing.T) {
	s := NewStorage()

	_, err := s.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)

	_, err = s.Set(context.Background(), "abc123", "https://example.org", "user1")
	assert.ErrorIs(t, err, ErrDuplicateKey)

	value, err := s.Get("abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", value, "the original value must not be overwritten")
}

func TestStorage_SetBatch(t *testing.T) {
	s := NewStorage()

	items := []URLRecord{
		{ID: "abc123", OriginURL: "https://example.com"},
		{ID: "def456", OriginURL: "https://example.org"},
	}

	err := s.SetBatch(context.Background(), items, "user1")
	require.NoError(t, err)

	for _, item := range items {
		value, err := s.Get(item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.OriginURL, value)
	}
}

func TestStorage_SetBatch_StopsOnFirstError(t *testing.T) {
	s := NewStorage()
	_, err := s.Set(context.Background(), "abc123", "https://existing.com", "user1")
	require.NoError(t, err)

	items := []URLRecord{
		{ID: "abc123", OriginURL: "https://example.com"}, // collides
		{ID: "def456", OriginURL: "https://example.org"},
	}

	err = s.SetBatch(context.Background(), items, "user1")
	assert.ErrorIs(t, err, ErrDuplicateKey)

	_, err = s.Get("def456")
	assert.ErrorIs(t, err, ErrNotFound, "batch should have stopped before storing the item after the failed one")
}

func TestStorage_GetUrlsByUser_NotImplemented(t *testing.T) {
	s := NewStorage()

	_, err := s.GetUrlsByUser("user1")
	assert.Error(t, err)
}

func TestStorage_SetDeletedBatch(t *testing.T) {
	s := NewStorage()
	_, err := s.Set(context.Background(), "abc123", "https://example.com", "user1")
	require.NoError(t, err)
	_, err = s.Set(context.Background(), "def456", "https://example.org", "user1")
	require.NoError(t, err)

	err = s.SetDeletedBatch(context.Background(), []string{"abc123"}, "user1")
	require.NoError(t, err)

	_, err = s.Get("abc123")
	assert.ErrorIs(t, err, ErrNotFound)

	value, err := s.Get("def456")
	require.NoError(t, err)
	assert.Equal(t, "https://example.org", value, "keys not in the batch must be left alone")
}
