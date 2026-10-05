package tools

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTTLCacheReturnsCachedValueWithinTTL(t *testing.T) {
	c := NewTTLCache[string, int]()
	calls := 0
	fetch := func() (int, error) { calls++; return calls, nil }

	v1, err := c.Get("k", time.Hour, fetch)
	require.NoError(t, err)
	v2, err := c.Get("k", time.Hour, fetch)
	require.NoError(t, err)

	assert.Equal(t, 1, v1)
	assert.Equal(t, 1, v2)
	assert.Equal(t, 1, calls)
}

func TestTTLCacheRefetchesAfterTTL(t *testing.T) {
	c := NewTTLCache[string, int]()
	calls := 0
	fetch := func() (int, error) { calls++; return calls, nil }

	_, _ = c.Get("k", 0, fetch)
	v, err := c.Get("k", 0, fetch)

	require.NoError(t, err)
	assert.Equal(t, 2, v)
}

func TestTTLCacheDoesNotCacheErrors(t *testing.T) {
	c := NewTTLCache[string, int]()

	_, err := c.Get("k", time.Hour, func() (int, error) { return 0, errors.New("boom") })
	require.Error(t, err)
	v, err := c.Get("k", time.Hour, func() (int, error) { return 42, nil })

	require.NoError(t, err)
	assert.Equal(t, 42, v)
}

func TestTTLCacheConcurrentCallersShareOneFetch(t *testing.T) {
	c := NewTTLCache[string, int]()
	var calls atomic.Int32
	release := make(chan struct{})
	fetch := func() (int, error) {
		calls.Add(1)
		<-release
		return 7, nil
	}

	var wg sync.WaitGroup
	results := make([]int, 10)
	for i := range results {
		wg.Go(func() {
			v, err := c.Get("k", time.Hour, fetch)
			assert.NoError(t, err)
			results[i] = v
		})
	}
	time.Sleep(20 * time.Millisecond) // let every caller reach the cache
	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), calls.Load())
	for _, v := range results {
		assert.Equal(t, 7, v)
	}
}

// A slow fetch must not block the other keys (e.g. one unreachable Helm
// repository must not stall every catalog).
func TestTTLCacheSlowFetchDoesNotBlockOtherKeys(t *testing.T) {
	c := NewTTLCache[string, int]()
	release := make(chan struct{})
	defer close(release)
	go func() {
		_, _ = c.Get("slow", time.Hour, func() (int, error) { <-release; return 0, nil })
	}()
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		_, _ = c.Get("fast", time.Hour, func() (int, error) { return 1, nil })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a slow fetch blocked another key")
	}
}

func TestTTLCachePanickingFetchReleasesWaiters(t *testing.T) {
	c := NewTTLCache[string, int]()

	assert.Panics(t, func() {
		_, _ = c.Get("k", time.Hour, func() (int, error) { panic("boom") })
	})
	v, err := c.Get("k", time.Hour, func() (int, error) { return 3, nil })

	require.NoError(t, err)
	assert.Equal(t, 3, v)
}
