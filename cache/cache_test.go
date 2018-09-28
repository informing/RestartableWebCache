// Black box testing of package cache.
package cache_test

import (
	"bytes"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/cache"
)

//TODO: Refactor tests into helper functions.

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

// checkError prints out and returns any non-nil errors.
func checkError(err error) (duplErr error) {
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	return err
}

// dirEmpty returns whether or not a given directory (name) is empty.
func dirEmpty(name string) (empty bool, err error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Try and read at max 1 file.
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		// The directory is empty.
		return true, nil
	}
	// Either not empty or error, suits both cases.
	return false, err
}

// Initialize our test files here.
// One of each of 700kB,500kB, and 200kB.
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
	// mounted a disk point <pwd>/test.

	currDir, err := os.Getwd()
	if checkError(err) != nil {
		t.Error("Error retrieving current working directory")
	}
	mountPath := filepath.Join(currDir, "test1")

	// If mountPath already exists as a folder, delete it.  We want
	// our call to cache.New to create the mount path.
	stat, err := os.Stat(mountPath)
	if !os.IsNotExist(err) && stat.IsDir() {
		if err = os.RemoveAll(mountPath); checkError(err) != nil {
			t.Error("Found already existing mount point and couldn't remove it.")
		}
	}

	testCache, err := cache.New("LFU", 1024, time.Duration(time.Second*3), mountPath)
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	// Make sure that the mountPath was created
	if _, err = os.Stat(mountPath); os.IsNotExist(err) {
		t.Errorf("Failed to create mount point for cache at %s", mountPath)
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

		// Sleep a bit to allow the disk save to run.
		time.Sleep(100 * time.Millisecond)

		fipath := filepath.Join(mountPath, cache.ToDiskString(testURL200))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL200))
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

		fipath := filepath.Join(mountPath, cache.ToDiskString(testURL200))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL200))
		}

		fipath = filepath.Join(mountPath, cache.ToDiskString(testURL500))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL500))
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

		if empty, err := dirEmpty(mountPath); checkError(err) != nil || !empty {
			t.Error("Mount path should be empty but is not")
		}
	})

	// Clean up folders we created for testing.
	if err = os.RemoveAll(mountPath); checkError(err) != nil {
		return
	}
}

func TestLRU(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of an hour (so nothing expires -
	// testing purposes) mounted at disk point /test.
	currDir, err := os.Getwd()
	if checkError(err) != nil {
		t.Error("Error retrieving current working directory")
	}
	mountPath := filepath.Join(currDir, "test2")

	// If mountPath already exists as a folder, delete it.  We want
	// our call to cache.New to create the mount path.
	stat, err := os.Stat(mountPath)
	if !os.IsNotExist(err) && stat.IsDir() {
		if err = os.RemoveAll(mountPath); checkError(err) != nil {
			t.Error("Found already existing mount point and couldn't remove it.")
		}
	}

	lruCache, err := cache.New("LRU", 1024, time.Duration(time.Hour*1), mountPath)
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	t.Run("Add overflowing item to cache, see that the correct LRU item is evicted", func(t *testing.T) {
		if err = lruCache.Save(testURL700, &testBuffer700); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL700.String())
		}
		time.Sleep(1 * time.Second)

		if err = lruCache.Save(testURL200, &testBuffer200); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL200.String())
		}
		time.Sleep(1 * time.Second)

		// This should evict the lru item - testBuffer700
		if err = lruCache.Save(testURL500, &testBuffer500); checkError(err) != nil {
			t.Errorf("Couldn't save %s to the cache", testURL500.String())
		}
		time.Sleep(1 * time.Second)

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

		// Sleep to allow disk saves to normalize.
		time.Sleep(1 * time.Second)

		fipath := filepath.Join(mountPath, cache.ToDiskString(testURL200))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL200))
		}

		fipath = filepath.Join(mountPath, cache.ToDiskString(testURL700))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL700))
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

		// Sleep to allow disk saves to normalize.
		time.Sleep(100 * time.Millisecond)

		fipath := filepath.Join(mountPath, cache.ToDiskString(testURL900))
		if _, err = os.Stat(fipath); os.IsNotExist(err) {
			t.Errorf("%s was not found on disk", cache.ToDiskString(testURL900))
		}

		fipath = filepath.Join(mountPath, cache.ToDiskString(testURL500))
		if _, err = os.Stat(fipath); !os.IsNotExist(err) {
			t.Errorf("%s was found on disk, but should have been deleted", cache.ToDiskString(testURL500))
		}

		fipath = filepath.Join(mountPath, cache.ToDiskString(testURL700))
		if _, err = os.Stat(fipath); !os.IsNotExist(err) {
			t.Errorf("%s was found on disk, but should have been deleted", cache.ToDiskString(testURL700))
		}
	})

	// Clean up folders we created for testing.
	if err = os.RemoveAll(mountPath); checkError(err) != nil {
		return
	}
}

func TestLFU(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of an hour (so nothing expires -
	// testing purposes) mounted at disk point /cache.
	currDir, err := os.Getwd()
	if checkError(err) != nil {
		t.Error("Error retrieving current working directory")
	}
	mountPath := filepath.Join(currDir, "test3")

	lfuCache, err := cache.New("LFU", 1024, time.Duration(time.Second*3), mountPath)
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

	// Clean up folders we created for testing.
	if err = os.RemoveAll(mountPath); checkError(err) != nil {
		return
	}
}

func TestLoadFromDisk(t *testing.T) {
	// Instantiate an LFU cache, with 1kB of storage, item expiry of an hour (so nothing expires -
	// testing purposes) mounted at disk point /test.
	currDir, err := os.Getwd()
	if checkError(err) != nil {
		t.Error("Error retrieving current working directory")
	}
	mountPath := filepath.Join(currDir, "test4")

	// If mountPath already exists as a folder, delete it.
	stat, err := os.Stat(mountPath)
	if !os.IsNotExist(err) && stat.IsDir() {
		if err = os.RemoveAll(mountPath); checkError(err) != nil {
			t.Error("Found already existing mount point and couldn't remove it.")
		}
	}

	// Make empty folder at mountPath.
	if err = os.Mkdir(mountPath, os.ModePerm); checkError(err) != nil {
		return
	}

	testItems := []struct {
		u url.URL
		b bytes.Buffer
	}{
		{
			testURL200,
			testBuffer200,
		},
		{
			testURL500,
			testBuffer500,
		},
		{
			testURL700,
			testBuffer700,
		},
		{
			testURL900,
			testBuffer900,
		},
	}

	var wg sync.WaitGroup
	for _, item := range testItems {
		wg.Add(1)
		// Avoid shadowing - gotcha!
		item := item
		go func() {
			defer wg.Done()
			savePath := filepath.Join(mountPath, cache.ToDiskString(item.u))
			toSave, err := os.Create(savePath)
			if checkError(err) != nil {
				return
			}
			defer toSave.Close()

			// Copy contents from our resource to a file.
			if _, err = io.Copy(toSave, &item.b); checkError(err) != nil {
				return
			}

			// Flush file contents to disk.
			if err = toSave.Sync(); checkError(err) != nil {
				return
			}
		}()
	}

	// Wait for all files to be saved to disk.
	// We can now run our test!
	wg.Wait()

	lruCache, err := cache.New("LRU", 1024, time.Duration(time.Hour*1), mountPath)
	if checkError(err) != nil {
		t.Error("Couldn't instantiate cache")
	}

	t.Run("Cache should fill up with files from the disk", func(t *testing.T) {
		if lruCache.Size() > 1024 {
			t.Errorf("Overfilled cache on startup.  Cache size: %d", lruCache.Size())
		}

		// The cache can be filled, with the available permutations, to either 700 or 900.
		// Assert that size here.
		if lruCache.Size() != 700 && lruCache.Size() != 900 {
			t.Errorf("Cache was filled on startup to an impossible size.  Cache size: %d", lruCache.Size())
		}
	})

	// Clean up folders we created for testing.
	if err = os.RemoveAll(mountPath); checkError(err) != nil {
		return
	}
}
