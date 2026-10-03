package appquery

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type compactCacheResult struct {
	digest   string
	count    int64
	revision string
}
type compactEntry struct {
	result  compactCacheResult
	expires time.Time
}
type compactFlight struct {
	done   chan struct{}
	result compactCacheResult
	err    error
}
type compactExperimentCache struct {
	mu      sync.Mutex
	max     int
	entries map[string]compactEntry
	order   []string
	flights map[string]*compactFlight
}

func newCompactExperimentCache(max int) *compactExperimentCache {
	return &compactExperimentCache{max: max, entries: map[string]compactEntry{}, flights: map[string]*compactFlight{}}
}

// Test-only digest/count/revision cache; all callers still need current
// Session, authorization and query context validation outside this object.
func (c *compactExperimentCache) get(ctx context.Context, key string, now time.Time, compute func(context.Context) (compactCacheResult, error)) (compactCacheResult, error) {
	c.mu.Lock()
	if cached, ok := c.entries[key]; ok && now.Before(cached.expires) {
		c.mu.Unlock()
		return cached.result, nil
	}
	if flight, ok := c.flights[key]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return compactCacheResult{}, ctx.Err()
		case <-flight.done:
			return flight.result, flight.err
		}
	}
	flight := &compactFlight{done: make(chan struct{})}
	c.flights[key] = flight
	c.mu.Unlock()
	flight.result, flight.err = compute(ctx)
	c.mu.Lock()
	if flight.err == nil && c.max > 0 {
		if _, present := c.entries[key]; !present {
			c.order = append(c.order, key)
		}
		c.entries[key] = compactEntry{result: flight.result, expires: now.Add(5 * time.Second)}
		for len(c.entries) > c.max {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
	}
	delete(c.flights, key)
	close(flight.done)
	c.mu.Unlock()
	return flight.result, flight.err
}
func (c *compactExperimentCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func TestCompactExperimentCacheScopeAndSingleFlight(t *testing.T) {
	c := newCompactExperimentCache(32)
	var calls atomic.Int64
	now := time.Unix(0, 0)
	compute := func(context.Context) (compactCacheResult, error) {
		calls.Add(1)
		return compactCacheResult{"abc", 2, "data=7;policy=3;schema=2;source=5"}, nil
	}
	key := func(actor, mask, criteria, revision string) string {
		return fmt.Sprint("app/table/view|", actor, "|", mask, "|", criteria, "|", revision)
	}
	common := key("actorA", "all:text;own:secret", "number>0", "data=7;policy=3;schema=2;source=5")
	for i := 0; i < 200; i++ {
		if got, err := c.get(context.Background(), common, now, compute); err != nil || got.count != 2 {
			t.Fatal(got, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("same canonical key recomputed %d times", calls.Load())
	}
	for _, different := range []string{
		key("actorB", "all:text;own:secret", "number>0", "data=7;policy=3;schema=2;source=5"),
		key("actorA", "all:text;all:secret", "number>0", "data=7;policy=3;schema=2;source=5"),
		key("actorA", "all:text;own:secret", "number>1", "data=7;policy=3;schema=2;source=5"),
		key("actorA", "all:text;own:secret", "number>0", "data=8;policy=3;schema=2;source=5"),
	} {
		if _, err := c.get(context.Background(), different, now, compute); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 5 {
		t.Fatalf("actor/mask/criteria/revision crossed cache boundary: calls=%d", calls.Load())
	}
	// An expired entry must be recomputed. This uses a supplied clock, not sleep.
	if _, err := c.get(context.Background(), common, now.Add(6*time.Second), compute); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 6 {
		t.Fatalf("expired cache entry reused: %d", calls.Load())
	}
	for i := 0; i < 200; i++ {
		if _, err := c.get(context.Background(), fmt.Sprintf("distinct-%03d", i), now, compute); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 206 || c.size() > 32 {
		t.Fatalf("distinct keys or bound wrong: calls=%d size=%d", calls.Load(), c.size())
	}

	// A separate cache with 200 concurrent consumers proves one in-flight
	// compute; no Session context or result rows are stored in this object.
	simultaneous := newCompactExperimentCache(32)
	var simultaneousCalls atomic.Int64
	var computeStarted sync.Once
	start := make(chan struct{})
	startedCompute := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := simultaneous.get(context.Background(), common, now, func(context.Context) (compactCacheResult, error) {
				simultaneousCalls.Add(1)
				computeStarted.Do(func() { close(startedCompute) })
				<-release
				return compactCacheResult{"abc", 2, "r"}, nil
			})
			if e != nil {
				t.Error(e)
			}
		}()
	}
	close(start)
	// The first compute reaches this gate; additional callers may join its
	// flight or read its completed cache entry after release.
	select {
	case <-startedCompute:
	case <-time.After(time.Second):
		t.Fatal("compute did not start")
	}
	close(release)
	wg.Wait()
	if simultaneousCalls.Load() != 1 {
		t.Fatalf("single-flight ran %d computes", simultaneousCalls.Load())
	}
}
