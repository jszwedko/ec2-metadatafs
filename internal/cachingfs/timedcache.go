// Ported from github.com/hanwen/go-fuse's unionfs package (BSD-licensed),
// which was removed when upgrading to go-fuse v2.
package cachingfs

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data interface{}

	// expiry is the absolute timestamp of the expiry.
	expiry time.Time
}

// timedCacheFetcher fetches the value for a cache miss
type timedCacheFetcher func(name string) (value interface{}, cacheable bool)

// timedCache caches the result of fetch() for some time. It is
// thread-safe. Calls of fetch() do not happen inside a critical
// section, so when multiple concurrent Get()s happen for the same
// key, multiple fetch() calls may be issued for the same key.
type timedCache struct {
	fetch timedCacheFetcher

	// ttl is the duration of the cache.
	ttl time.Duration

	cacheMapMutex sync.RWMutex
	cacheMap      map[string]*cacheEntry
}

// newTimedCache creates a new cache with the given TTL. If ttl <= 0, the
// caching is indefinite.
func newTimedCache(fetcher timedCacheFetcher, ttl time.Duration) *timedCache {
	return &timedCache{
		fetch:    fetcher,
		ttl:      ttl,
		cacheMap: make(map[string]*cacheEntry),
	}
}

func (c *timedCache) Get(name string) interface{} {
	c.cacheMapMutex.RLock()
	info, ok := c.cacheMap[name]
	c.cacheMapMutex.RUnlock()

	valid := ok && (c.ttl <= 0 || info.expiry.After(time.Now()))
	if valid {
		return info.data
	}
	return c.getFresh(name)
}

func (c *timedCache) set(name string, val interface{}) {
	c.cacheMapMutex.Lock()
	defer c.cacheMapMutex.Unlock()

	c.cacheMap[name] = &cacheEntry{
		data:   val,
		expiry: time.Now().Add(c.ttl),
	}
}

func (c *timedCache) getFresh(name string) interface{} {
	data, ok := c.fetch(name)
	if ok {
		c.set(name, data)
	}
	return data
}
