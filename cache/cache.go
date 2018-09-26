package cache

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/url"
	"sync"
	"time"
)

func init() {
	// Give us line numbers!
	log.SetFlags(log.Lshortfile)
}

// If error is non-nil, print it out and return it.
func checkError(err error) (duplErr error) {
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	return err
}

// ErrBadReplacementPolicy signifies that an incorrect replacement policy was specified.
// ErrCacheSizeExceeded means that an attempt to add a resource to the cache caused a size overflow.
var (
	ErrBadReplacementPolicy   = errors.New("Bad replacement policy: must be one of 'LRU' or 'LFU'")
	ErrCacheSizeExceeded      = errors.New("Maximum cache size exceeded")
	ErrResourceNotInCache     = errors.New("Requested resource was not found in cache")
	ErrCouldntReadResourceLen = errors.New("Couldnt read length of requested resource")
)

// Cache is a generic cache interface type.
// All operations on Cache should be thread safe.
type Cache interface {
	// Get retrieves a resource from the cache.
	Get(url url.URL) (io.ReadWriter, error)

	// Save saves a resource to the cache.
	Save(url url.URL, fi io.ReadWriter) error

	// Size returns the current size of the cache (not the max size).
	Size() int
}

// lru is an implementation of an LRU cache that satisfies interface Cache.
type lru struct {
	*memoryCache
}

// lfu is an implementation of an LFU cache that satisfies interface Cache.
type lfu struct {
	*memoryCache
}

// memoryCache is an in memory cache with basic utility functions.
// Files are purged after expiration seconds.  The cache has maxSize maxSize
// and current size size.  It is internally modelled by a hashmap.
type memoryCache struct {
	maxSize    int64 // Use int64 because os.File stores its size metric as int64
	size       int64 // Same as above
	expiration time.Duration
	memory     map[url.URL]*resource
	mountPath  string
	sync.Mutex
}

// resource is a cache-item. It contains a saveTime, which is
// the time at which this resource was entered into the cache.
// It also contains an access count, which is the number of times
// this resource has been accessed via the cache.  Both of these metrics
// are useful for implemented LRU / LFU replacement policies
type resource struct {
	file        io.ReadWriter // Use an io.ReadWriter here (so we can use both *os.File and *bytes.Buffer).
	saveTime    time.Time
	accessCount int
}

// fileSize returns the size, in bytes, of fi.
// fi should be of type *bytes.Buffer.
func fileSize(fi io.ReadWriter) (size int64, err error) {
	// We can get the length of a *bytes.Buffer, make the assertion here.
	if fi, ok := fi.(*bytes.Buffer); ok {
		return int64(fi.Len()), nil
	}
	// TODO: Add support for *os.File if needed.
	return 0, ErrCouldntReadResourceLen
}

// purgeExpired purges expired resources from the cache.
// Resources in memory are deleted immediately, and a goroutine
// is dispatched to delete the item from disk.
func (cache *memoryCache) purgeExpired() {
	// Go through all cache items.
	for url, resource := range cache.memory {
		if time.Since(resource.saveTime) > cache.expiration {
			// This file has expired.  Delete this resource.
			if err := cache.deleteResource(url); err != nil {
				// If there was an error deleting this resource,
				// move on to the next potentially expired cache-item.
				continue
			}
		}
	}
	return
}

// saveResource saves fi to cache. Files are saved immediately to the in-memory cache,
// and a goroutine is dispatched to save the file to disk.  If fi won't fit in the cache,
// resources are removed from cache until fi can be saved.  The provided function argument
// nextToGo determines which resource is the next item to be removed from the cache.
func (cache *memoryCache) saveResource(url url.URL, fi io.ReadWriter, nextToGo func(cache *memoryCache) url.URL) (err error) {

	// save tries to save fi of size fiSize to cache.  If it succeeds, return true,
	// if not, return false.
	save := func(fiSize int64, fi io.ReadWriter, cache *memoryCache) (fit bool) {
		// Make sure fi will fit in the cache.  Calculate the amount of space we need.
		needSize := fiSize + cache.size

		// If it fits, save it and return.
		if needSize <= cache.maxSize {
			cache.memory[url] = &resource{
				file:     fi,
				saveTime: time.Now(),
			}
			cache.size = needSize
			// TODO: size adjustments if resource already in cache
			// TODO: dispatch goroutine to save to disk
			return true
		}
		// It didn't fit.
		return false
	}

	// Get the size of fi.
	size, err := fileSize(fi)
	if err != nil {
		return err
	}

	// Before doing anything, try and see if fi fits in the cache.
	// If it does, we don't need to replace anything.
	var fits bool
	if fits = save(size, fi, cache); fits {
		// It fit, we're good, so return.
		return nil
	}

	// Start removing resources, one by one.  Try and save fi until it fits.
	for !fits {
		// It didn't fit, so get the next resource to remove and remove it.
		toRemove := nextToGo(cache)
		if err := cache.deleteResource(toRemove); err != nil {
			// There was an issue deleting this resource, continue to the next.
			continue
		}
		// Try and save fi again, hopefully we freed up enough space.
		fits = save(size, fi, cache)
	}

	return nil
}

func (cache *memoryCache) deleteResource(url url.URL) (err error) {
	if resource, ok := cache.memory[url]; ok {
		// The resource exists, we can delete it.
		// Get its size, subtract that value from the total size,
		// and delete is from memory.  Also dispatch a goroutine to delete from disk.
		size, err := fileSize(resource.file)
		if err != nil {
			return err
		}
		cache.size -= size
		delete(cache.memory, url)
		// TODO: dispatch goroutine to delete from disk
		return nil
	}
	// If the resource doesn't exist, don't error - just no-op.
	return nil
}

// getResource retrieves the file saved in the cache by url.
// Everytime a resource is retrieved, its accessCount increments by 1.
// If the resource specified by url does not exist in the cache, an appropriate error
// is returned.
func (cache *memoryCache) getResource(url url.URL) (fi io.ReadWriter, err error) {
	if resource, ok := cache.memory[url]; ok {
		// The resource is here; increment its accessCount and return it.
		// Also, set its saveTime to time.Now().
		resource.accessCount++
		resource.saveTime = time.Now()
		return resource.file, nil
	}
	// Resource was not found, error.
	return nil, ErrResourceNotInCache
}

// getLFU finds the LFU used item in cache, and returns its url.
func getLFU(cache *memoryCache) (lfuURL url.URL) {
	var lfu int
	// Initialize our lfu to the first item of the map.
	for url, resource := range cache.memory {
		lfu = resource.accessCount
		lfuURL = url
		break
	}

	// Find the lfu resource, and return that url.
	for url, resource := range cache.memory {
		if resource.accessCount < lfu {
			lfu = resource.accessCount
			lfuURL = url
		}
	}
	return lfuURL
}

// getLRU finds the LRU item in cache, and returns its url.
func getLRU(cache *memoryCache) (lruURL url.URL) {
	var lruTime time.Time
	// Initialize our lruTime to the first item of the map.
	for url, resource := range cache.memory {
		lruTime = resource.saveTime
		lruURL = url
		break
	}

	// Find the lru resource, and return that url.
	for url, resource := range cache.memory {
		if resource.saveTime.Before(lruTime) {
			lruTime = resource.saveTime
			lruURL = url
		}
	}
	return lruURL
}

// getSize retrieves the current size of cache.
func (cache *memoryCache) getSize() (size int64) {
	return cache.size
}

// New returns a new cache with policy policy, max size size, and item expiration time
// expiration.
func New(policy string, size int, expiration time.Duration, mountPath string) (cache Cache, err error) {
	memCache := &memoryCache{
		maxSize:    int64(size),
		expiration: expiration,
		memory:     make(map[url.URL]*resource),
		mountPath:  mountPath,
	}
	switch policy {
	case "LRU":
		cache = &lru{memoryCache: memCache}
	case "LFU":
		cache = &lfu{memoryCache: memCache}
	default:
		// Incorrect cache replacement policy; return an error.
		err = ErrBadReplacementPolicy
		return
	}

	// Spin up a gouroutine to purge expired items every 100ms.
	// This is concurrency safe.
	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				memCache.Lock()
				memCache.purgeExpired()
				memCache.Unlock()
			}
		}
	}()
	return
}

// Get implements Cache.Get for an LRU cache.
func (cache *lru) Get(url url.URL) (fi io.ReadWriter, err error) {
	cache.Lock()
	defer cache.Unlock()

	fi, err = cache.getResource(url)
	return
}

// Save implements Cache.Save for an LRU cache.
func (cache *lru) Save(url url.URL, fi io.ReadWriter) (err error) {
	cache.Lock()
	defer cache.Unlock()

	err = cache.saveResource(url, fi, getLRU)
	return
}

// Size gets the current size of the LRU cache (not max size).
func (cache *lru) Size() (size int) {
	cache.Lock()
	defer cache.Unlock()

	return int(cache.getSize())
}

// Get implements Cache.Get for an LFU cache.
func (cache *lfu) Get(url url.URL) (fi io.ReadWriter, err error) {
	cache.Lock()
	defer cache.Unlock()

	fi, err = cache.getResource(url)
	return
}

// Save implements Cache.Save for an LFU cache.
func (cache *lfu) Save(url url.URL, fi io.ReadWriter) (err error) {
	cache.Lock()
	defer cache.Unlock()

	err = cache.saveResource(url, fi, getLFU)
	return
}

// Size gets the current size of the LFU cache (not max size).
func (cache *lfu) Size() (size int) {
	cache.Lock()
	defer cache.Unlock()

	return int(cache.getSize())
}
