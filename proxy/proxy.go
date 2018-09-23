package proxy

import (
	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/cache"
)

// Proxy ...
type Proxy struct {
	ipPort string
	cache  cache.Cache
}

// For now, the default proxy serves as a singleton.
// We could extend this package to include an implementation
// of proxy.New and receiver methods on multiple instances of Proxy,
// but that is out of the scope of what is needed for A2.
var defaultProxy = &Proxy{}

// ListenOn ...
func ListenOn(ipPort string) { defaultProxy.ipPort = ipPort }

// UseCache sets the default proxy to use cache cache.
func UseCache(cache cache.Cache) { defaultProxy.cache = cache }

// InterceptGET ...
func InterceptGET() (err error) {
	// Some shit here.
	return
}
