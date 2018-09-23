package cache

import (
	"errors"
	"net/url"
	"os"
	"time"
)

// ErrBadReplacementPolicy signifies that an incorrect replacement policy was specified.
var ErrBadReplacementPolicy = errors.New("Bad replacement policy: must be one of 'LRU' or 'LFU'")

// Cache is a generic cache interface type.
type Cache interface {
	// Get retrieves a resource from the cache.
	Get(url url.URL) (string, error)

	// Save saves a resource to the cache.
	Save(url url.URL) error
}

// lru is an implementation of an LRU cache.
type lru struct {
	*memoryCache
	mountPath string
}

// lfu is an implementation of an LFU cache.
type lfu struct {
	*memoryCache
	mountPath string
}

// memoryCache is an in memory cache with basic utility functions.
type memoryCache struct {
	size       int
	expiration time.Duration
	memory     map[url.URL]resource
}

// resource is a cache-item.
type resource struct {
	file     *os.File
	saveTime time.Time
}

func (cache *memoryCache) purgeExpired() (err error) {
	return errors.New("None")
}

func (cache *memoryCache) saveResource() (err error) {
	return errors.New("None")
}

func (cache *memoryCache) getResource() (err error) {
	return errors.New("None")
}

// New returns a new cache with policy policy, max size size, and item expiration time
// expiration.
func New(policy string, size int, expiration time.Duration, mountPath string) (cache Cache, err error) {
	err = nil
	memCache := &memoryCache{
		size:       size,
		expiration: expiration,
		memory:     make(map[url.URL]resource),
	}
	switch policy {
	case "LRU":
		cache = &lru{
			memCache,
			mountPath,
		}
	case "LFU":
		cache = &lfu{
			memCache,
			mountPath,
		}
	default:
		// Incorrect cache replacement policy; return an error.
		cache = nil
		err = ErrBadReplacementPolicy
	}
	return cache, err
}

// Get implements Cache.Get
func (cache *lru) Get(url url.URL) (string, error) { return "", errors.New("None") }

// Save implements Cache.Save
func (cache *lru) Save(url url.URL) error { return errors.New("None") }

// Get implements Cache.Get
func (cache *lfu) Get(url url.URL) (string, error) { return "", errors.New("None") }

// Save implements Cache.Save
func (cache *lfu) Save(url url.URL) error { return errors.New("None") }
