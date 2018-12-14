# RestartableWebCache
📸🖹🌍 A web cache that inter-operates with Firefox and dumps cached contents on disk.

## Features
- Consistent with the cache-control directives in the HTTP header field
- Able to serve multiple clients concurrently and has persistent state to recover from crashes or restarts
- Parses the HTML content from HTTP responses and rewrites URLs to content that it cached
- Caches and serves static web content retrieved by a browser using HTTP GETs
- Deletes cached items from both memory and disk once they expire

## Usage
```sh
go run web-cache.go [ip1:port1] [ip2:port2] [replacement_policy] [cache_size] [expiration_time]
```
1. `[ip1:port1]`: The TCP IP address and the port that the web cache will bind to to accept connections from clients. 
2. `[ip2:port2]`: The TCP IP address and the port that the web cache should use when rewriting the HTML. For example, the web cache would rewrite `<img src="http://foo.com/image.jpg"/>` to `<img src="http://ip2:port2/URL"/>`
3. `[replacement_policy]`: The replacement policy (`LRU` or `LFU`) that the web cache follows during eviction.
4. `[cache_size]`: The capacity of the cache in MB (your cache cannot use more than this amount of capacity). Note that this specifies the (same) capacity for both the memory cache and the disk cache.
5. `[expiration_time]`: The time period in seconds after which an item in the cache is considered to be expired.

## Environment
- The web cache code runs with Go 1.9.7
- Only uses standard library Go packages and the HTML library for parsing HTML in the web cache
