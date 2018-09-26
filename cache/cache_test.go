// Black box testing of package cache.
package cache_test

import (
	"bytes"
	"log"
	"net/url"
	"testing"
	"time"

	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/cache"
)

var (
	testURL200    url.URL
	testURL500    url.URL
	testURL700    url.URL
	testURL900    url.URL
	testBuffer200 bytes.Buffer
	testBuffer500 bytes.Buffer
	testBuffer700 bytes.Buffer
	testBuffer900 bytes.Buffer
)

// If error is non-nil, print it out and return it.
func checkError(err error) (duplErr error) {
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	return err
}

// Initialize our test files here.
// One of each of 700kB, 500kB, and 200kB.
func init() {
	url200, err := url.Parse("/test/200")
	if checkError(err) != nil {
		return
	}
	testURL200 = *url200
	bytes200 := make([]byte, 200)
	if _, err = testBuffer200.Write(bytes200); checkError(err) != nil {
		return
	}

	url500, err := url.Parse("/test/500")
	if checkError(err) != nil {
		return
	}
	testURL500 = *url500
	bytes500 := make([]byte, 500)
	if _, err = testBuffer500.Write(bytes500); checkError(err) != nil {
		return
	}

	url700, err := url.Parse("/test/700")
	if checkError(err) != nil {
		return
	}
	testURL700 = *url700
	bytes700 := make([]byte, 700)
	if _, err = testBuffer700.Write(bytes700); checkError(err) != nil {
		return
	}

	url900, err := url.Parse("/test/900")
	if checkError(err) != nil {
		return
	}
	testURL900 = *url900
	bytes900 := make([]byte, 900)
	if _, err = testBuffer900.Write(bytes900); checkError(err) != nil {
		return
	}
}

func TestCache(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of three seconds (for testing)
	// mounted a disk point /cache.
	testCache, err := cache.New("LFU", 1024, time.Duration(time.Second*3), "/cache")
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	t.Run("Try and save and retrieve from the cache", func(t *testing.T) {
		if err = testCache.Save(testURL200, &testBuffer200); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL200.String())
		}

		buf, err := testCache.Get(testURL200)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL200.String())
		}

		if buf != &testBuffer200 {
			t.Errorf("Failed to retrieve %s from the cache", testURL200.String())
		}

		if testCache.Size() != testBuffer200.Len() {
			t.Errorf("Size mismatch: cache should have size 200 bytes but has size %d", testBuffer200.Len())
		}
	})

	t.Run("Try to save and retrieve two things from the cache", func(t *testing.T) {
		if err = testCache.Save(testURL500, &testBuffer500); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL500.String())
		}

		buf, err := testCache.Get(testURL200)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL200.String())
		}

		if buf != &testBuffer200 {
			t.Errorf("Failed to retrieve %s from the cache", testURL200.String())
		}

		time.Sleep(2 * time.Second)
		buf2, err := testCache.Get(testURL500)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL500.String())
		}

		if buf2 != &testBuffer500 {
			t.Errorf("Failed to retrieve %s from the cache", testURL500.String())
		}

		if testCache.Size() != testBuffer200.Len()+testBuffer500.Len() {
			t.Errorf("Size mismatch: cache should have size 700 bytes but has size %d", testCache.Size())
		}
	})

	t.Run("Save an item to the cache and let it expire, see that its gone", func(t *testing.T) {
		// Wait until testURL200 expires.
		time.Sleep(2 * time.Second)
		_, err := testCache.Get(testURL200)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}
		if testCache.Size() != testBuffer500.Len() {
			t.Errorf("Size mismatch: cache should have size 500 bytes but has size %d", testCache.Size())
		}

		buf2, err := testCache.Get(testURL500)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL500.String())
		}

		if buf2 != &testBuffer500 {
			t.Errorf("Failed to retrieve %s from the cache", testURL500.String())
		}

		// Wait until testURL500 expires.
		time.Sleep(4 * time.Second)
		_, err = testCache.Get(testURL500)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}

		if testCache.Size() != 0 {
			t.Errorf("Size mismatch: cache should have size 0 bytes but has size %d", testCache.Size())
		}
	})
}

func TestLRU(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of an hour (so nothing expires -
	// testing purposes) mounted at disk point /cache.
	lruCache, err := cache.New("LRU", 1024, time.Duration(time.Hour*1), "/cache")
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	t.Run("Add overflowing item to cache, see that the correct LRU item is evicted", func(t *testing.T) {
		if err = lruCache.Save(testURL700, &testBuffer700); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL700.String())
		}

		if err = lruCache.Save(testURL200, &testBuffer200); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL200.String())
		}

		// This should evict the lru item - testBuffer700
		if err = lruCache.Save(testURL500, &testBuffer500); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL500.String())
		}

		if lruCache.Size() != testBuffer200.Len()+testBuffer500.Len() {
			t.Errorf("Size mismatch: cache should have size 700 bytes but has size %d", lruCache.Size())
		}

		_, err = lruCache.Get(testURL700)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found unexpected resource in cache")
		}

		buf, err := lruCache.Get(testURL500)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL500.String())
		}

		if buf != &testBuffer500 {
			t.Errorf("Failed to retrieve %s from the cache", testURL500.String())
		}

		buf2, err := lruCache.Get(testURL200)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL200.String())
		}

		if buf2 != &testBuffer200 {
			t.Errorf("Failed to retrieve %s from the cache", testURL200.String())
		}

		// Should evict testBuffer500, new size should be 200 + 700 = 900
		if err = lruCache.Save(testURL700, &testBuffer700); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL700.String())
		}

		_, err = lruCache.Get(testURL500)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found unexpected resource in cache")
		}

		if lruCache.Size() != testBuffer200.Len()+testBuffer700.Len() {
			t.Errorf("Size mismatch: cache should have size 900 bytes but has size %d", lruCache.Size())
		}
	})

	t.Run("Add largely overflowing item to cache, see that the correct two LRU items are evicted", func(t *testing.T) {
		// Adding testBuffer900 should evict both of testBuffer200 and testBuffer700
		if err = lruCache.Save(testURL900, &testBuffer900); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL900.String())
		}

		_, err = lruCache.Get(testURL200)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}

		_, err = lruCache.Get(testURL700)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}

		if lruCache.Size() != testBuffer900.Len() {
			t.Errorf("Size mismatch: cache should have size 900 bytes but has size %d", lruCache.Size())
		}
	})
}

func TestLFU(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of an hour (so nothing expires -
	// testing purposes) mounted at disk point /cache.
	lfuCache, err := cache.New("LFU", 1024, time.Duration(time.Second*3), "/cache")
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	t.Run("Add overflowing item to cache, see that the correct LFU item is evicted", func(t *testing.T) {
		if err = lfuCache.Save(testURL700, &testBuffer700); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL700.String())
		}

		if err = lfuCache.Save(testURL200, &testBuffer200); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL200.String())
		}

		// This should evict the lfu item - testBuffer700
		lfuCache.Get(testURL200)
		if err = lfuCache.Save(testURL500, &testBuffer500); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL500.String())
		}

		if lfuCache.Size() != testBuffer200.Len()+testBuffer500.Len() {
			t.Errorf("Size mismatch: cache should have size 700 bytes but has size %d", lfuCache.Size())
		}

		_, err = lfuCache.Get(testURL700)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found unexpected resource in cache")
		}

		buf, err := lfuCache.Get(testURL500)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL500.String())
		}

		if buf != &testBuffer500 {
			t.Errorf("Failed to retrieve %s from the cache", testURL500.String())
		}

		buf2, err := lfuCache.Get(testURL200)
		if checkError(err) != nil {
			t.Errorf("Couldn't retrieve %s from the cache", testURL200.String())
		}

		if buf2 != &testBuffer200 {
			t.Errorf("Failed to retrieve %s from the cache", testURL200.String())
		}

		// Cache hits:
		// testBuffer200 - 3
		// testBuffer500 - 2
		// Should evict testBuffer500, new size should be 200 + 700 = 900
		if err = lfuCache.Save(testURL700, &testBuffer700); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL700.String())
		}

		_, err = lfuCache.Get(testURL500)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found unexpected resource in cache")
		}

		if lfuCache.Size() != testBuffer200.Len()+testBuffer700.Len() {
			t.Errorf("Size mismatch: cache should have size 900 bytes but has size %d", lfuCache.Size())
		}
	})

	t.Run("Add largely overflowing item to cache, see that the correct two LFU items are evicted", func(t *testing.T) {
		// Cache hits:
		// testBuffer200 - 3
		// testBuffer700 - 1
		// Adding testBuffer900 should evict both of testBuffer200 and testBuffer700
		if err = lfuCache.Save(testURL900, &testBuffer900); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL900.String())
		}

		_, err = lfuCache.Get(testURL200)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}

		_, err = lfuCache.Get(testURL700)
		if checkError(err) != cache.ErrResourceNotInCache {
			t.Error("Found resource in cache when it should have expired")
		}

		if lfuCache.Size() != testBuffer900.Len() {
			t.Errorf("Size mismatch: cache should have size 900 bytes but has size %d", lfuCache.Size())
		}
	})
}
