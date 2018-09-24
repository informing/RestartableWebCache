package main

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/cache"
	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/proxy"
)

func init() {
	// Give us line numbers!
	log.SetFlags(log.Lshortfile)
}

// ErrInvalidArgs is an error signifying incorrectly supplied command line arguments.
var ErrInvalidArgs = errors.New("Invalid arguments supplied.  Usage:\n\tgo run web-cache.go [ip:port] [replacement_policy ('LRU' or 'LFU')] [cache_size (in MB)] [expiration_time]")

// If error is non-nil, print it out and return it.
func checkError(err error) (duplErr error) {
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}
	return err
}

// parseArgs parses and returns command line arguments supplied to the program.
// Arguments should be supplied in the format:
// go run web-cache.go [ip:port] [replacement_policy ("LRU" or "LFU")] [cache_size (in MB)] [expiration_time (seconds)]
// (As per A2 spec)
func parseArgs() (ipPort, replacementPolicy string, size int, expirationTime time.Duration, err error) {
	// If an incorrect length of arguments were specified, return and error and the zero-value
	// for the rest of the arguments.  We should have five arguments: the four specified above as well
	// as the file name of this file.
	if len(os.Args) != 5 {
		err = ErrInvalidArgs
		return
	}

	// We won't worry about precise argument validation for now - it would be nice in reality,
	// but in the scope of A2, is not truly necessary.  Instead, assume that the provided arguments
	// are correct. What we can do however, before moving forward, is to convert size from a string to an int,
	// and expiration time from a string to a time.Duration.

	// These two are already read from the cmd line as strings; easy.
	ipPort, replacementPolicy = os.Args[1], os.Args[2]

	// Otherwise, do the conversions.
	size, err = strconv.Atoi(os.Args[3])
	if checkError(err) != nil {
		return
	}

	expTimeInt, err := strconv.Atoi(os.Args[4])
	if checkError(err) != nil {
		return
	}

	// Create a time.Duration for expirationTime.
	expirationTime = time.Duration(expTimeInt) * time.Second
	log.Printf("Parsed command line arguments:\nIPPort: %s\nReplacement policy: %s\nMax cache size: %d\nCache item expiry time: %s\n",
		ipPort, replacementPolicy, size, expirationTime.String())
	return
}

// Entry point.
func main() {
	// Try and parse arguments from command line.
	ipPort, replacementPolicy, maxSize, expirationTime, err := parseArgs()
	if checkError(err) != nil {
		return
	}

	// Cache files on disk at root /cache.
	mountPath := "/cache"

	// Create a new cache.
	cache, err := cache.New(replacementPolicy, maxSize, expirationTime, mountPath)
	if checkError(err) != nil {
		return
	}

	// Start up our proxy server, transmitting through ipPort, and set up with
	// our newly configured cache.
	proxy.ListenOn(ipPort)
	proxy.UseCache(cache)
	proxy.InterceptGET()
}
