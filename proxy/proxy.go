package proxy

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.ugrad.cs.ubc.ca/CPSC416-2018W-T1/A2-i8b0b-e8y0b/cache"
	"golang.org/x/net/html"
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

func dumpLink(link string) (dumpedLink string) {
	return "http://" + defaultProxy.ipPort + "/?referrer='" + strings.Replace(link, "/", "-", -1) + "'"
}

func loadLink(link string) (loadedLink string) {
	components := strings.Split(link, "%27")
	if len(components) >= 2 {
		return strings.Replace(components[1], "-", "/", -1)
	} else {
		fmt.Println(components)
		return ""
	}
}

func cacheResource(resourceLink string) (cached bool) {
	// check if Resouce URI is relative
	debugPrompt := "func cacheResource:"
	if resourceLink[0] == '/' && !strings.HasPrefix(resourceLink, "//") {
		fmt.Println(debugPrompt, "will not parse relative links")
		return false
	} else if resourceLink[0] != '/' && (!strings.HasPrefix(resourceLink, "http://")) {
		// TODO: determine if we want to cache stuffs like "example.com/a.png"
		fmt.Println(debugPrompt, "will not parse relative links or load non-http resources")
		return false
	}
	response, err := http.Get(resourceLink)
	if err != nil {
		// Cannot find URI
		fmt.Println(debugPrompt, "cannot find the given resource", resourceLink)
		return false
	}
	responseBodyData, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	var responseBuffer bytes.Buffer
	responseBuffer.Write(responseBodyData)
	resourceURL, _ := url.Parse(resourceLink)

	fmt.Println(debugPrompt, "saving", resourceLink, "to cache")
	fmt.Println(debugPrompt, "... with header", response.Header)
	defaultProxy.cache.SaveWithHeaders(*resourceURL, &responseBuffer, response.Header)
	return true
}

func hash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return "[" + strconv.Itoa(int(h.Sum32())) + "]"
}

func serveDirectly(proxyWriter http.ResponseWriter, client *http.Client, clientRequest *http.Request) {
	proxyRequest, err := http.NewRequest(clientRequest.Method, clientRequest.RequestURI, clientRequest.Body)
	for name, value := range clientRequest.Header {
		proxyRequest.Header.Set(name, value[0])
	}
	serverResponse, err := client.Do(proxyRequest)
	defer clientRequest.Body.Close()

	if err != nil {
		http.Error(proxyWriter, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range serverResponse.Header {
		proxyWriter.Header().Set(k, v[0])
	}
	proxyWriter.WriteHeader(serverResponse.StatusCode)
	defer serverResponse.Body.Close()
	responseBodyData, _ := ioutil.ReadAll(serverResponse.Body)
	proxyWriter.Write(responseBodyData)
}

func serveAndCache(proxyWriter http.ResponseWriter, client *http.Client, clientRequest *http.Request) {
	resourceURL, _ := url.Parse(clientRequest.RequestURI)
	hashedLink := hash(clientRequest.RequestURI)

	fmt.Println("The requested resource is not in cache", hashedLink)
	proxyRequest, err := http.NewRequest(clientRequest.Method, clientRequest.RequestURI, clientRequest.Body)
	for name, value := range clientRequest.Header {
		proxyRequest.Header.Set(name, value[0])
	}
	serverResponse, err := client.Do(proxyRequest)
	fmt.Println("Received response from the server", hashedLink)

	if err != nil {
		http.Error(proxyWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	responseBodyData, err := ioutil.ReadAll(serverResponse.Body)
	if err != nil {
		fmt.Println(err)
		return
	} else {
		for k, v := range serverResponse.Header {
			// except for size
			if k != "Content-Length" {
				proxyWriter.Header().Set(k, v[0])
			}
		}
		proxyWriter.WriteHeader(serverResponse.StatusCode)

		var responseBuffer, parseBuffer bytes.Buffer
		multiWriter := io.MultiWriter(&parseBuffer, &responseBuffer)
		multiWriter.Write(responseBodyData)
		defer serverResponse.Body.Close()

		// responseBuffer.Write(responseBodyData)
		fmt.Println("Calling cache.Save to cache the server response")
		defaultProxy.cache.SaveWithHeaders(*resourceURL, bytes.NewBuffer(responseBuffer.Bytes()), serverResponse.Header)

		if strings.HasPrefix(serverResponse.Header.Get("Content-Type"), "text/html") {
			fmt.Println("Parsing the response body to find more resources to cache")
			lists, _ := ParseResponseBody(&parseBuffer, serverResponse.Header)
			fmt.Println("Going to replace:", lists)
			dumpedResponseData := responseBuffer.Bytes()
			for k, v := range lists {
				dumpedResponseData = bytes.Replace(dumpedResponseData, []byte(k), []byte(v), -1)
			}
			fmt.Println(string(dumpedResponseData))
			proxyWriter.Write(dumpedResponseData)
		}
	}
}

func serveWithCache(proxyWriter http.ResponseWriter, originalHeaders http.Header, cachedResponseBuffer *bytes.Buffer) {
	fmt.Println("Got the requested resource from cache")
	fmt.Println("Serving content to browser...")

	// Make a temporary copy of this cache resource.  We do not
	// want to drain the actual buffer in the cache.
	var tmp bytes.Buffer
	if _, err := tmp.Write(cachedResponseBuffer.Bytes()); err != nil {
		fmt.Println(err)
		return
	}

	// Send back the original buffers
	for k, v := range originalHeaders {
		proxyWriter.Header().Set(k, v[0])
	}
	if _, err := io.Copy(proxyWriter, &tmp); err != nil {
		fmt.Println(err)
	}
}

func handler(proxyWriter http.ResponseWriter, clientRequest *http.Request) {
	// var err error
	// var serverResponse *http.Response
	// var proxyRequest *http.Request
	client := &http.Client{}

	fmt.Println("Client requested", clientRequest.Method, clientRequest.RequestURI)

	if strings.HasPrefix(clientRequest.RequestURI, "http://") && clientRequest.Method == "GET" {
		// We only handle http GET requests
		// this is not a local/rewritten request
		hashedLink := hash(clientRequest.RequestURI)
		resourceURL, _ := url.Parse(clientRequest.RequestURI)
		fmt.Println("Trying to fetch resource from cache.Get", hashedLink)
		cachedResponse, originalHeaders, err := defaultProxy.cache.GetWithHeaders(*resourceURL)
		if err == cache.ErrResourceNotInCache {
			serveAndCache(proxyWriter, client, clientRequest)
		} else {
			serveWithCache(proxyWriter, originalHeaders, cachedResponse)
		}
	} else if strings.HasPrefix(clientRequest.RequestURI, "/?referrer") && clientRequest.Method == "GET" {
		// this is a local/rewritten request
		originalLink := loadLink(clientRequest.RequestURI)
		hashedLink := hash(originalLink)

		fmt.Println("... alias =", originalLink, hashedLink)

		resourceURL, _ := url.Parse(originalLink)
		fmt.Println("Trying to fetch resource from cache.Get", originalLink)
		cachedResponse, originalHeaders, err := defaultProxy.cache.GetWithHeaders(*resourceURL)
		if err == cache.ErrResourceNotInCache {
			// resouce not in cache should not happen, but we can deal with it
			fmt.Println("The requested resource is not in cache", hashedLink)
			serveAndCache(proxyWriter, client, clientRequest)
		} else {
			// resource is in cache and we can serve it
			serveWithCache(proxyWriter, originalHeaders, cachedResponse)
		}
	} else {
		// ... http POST and other stuffs go here
		fmt.Println("Cannot parse the provided URI, will simply serve w/o caching")
		serveDirectly(proxyWriter, client, clientRequest)
	}
}

// ParseResponseBody parses the given reader
func ParseResponseBody(r io.Reader, h http.Header) (lists map[string]string, err error) {
	// depth := 0
	z := html.NewTokenizer(r)

	// expectScript := false
	// expectLink := false
	links := make(map[string]string)
	for {
		tt := z.Next()
		// fmt.Println("Fetching next token", tt)
		switch tt {
		case html.ErrorToken:
			// Ultimately we will get to this point (EOF)
			return links, z.Err()
		case html.TextToken:
			continue
			/*
				link := make([]byte, 1024)
				if expectLink {
					copy(link, z.Text())
					cacheResource(string(link))
					expectLink = false
				} else if expectScript {
					cacheResource(string(link))
					expectScript = false
				}
			*/
		case html.SelfClosingTagToken:
			token := z.Token()
			if "img" == token.Data {
				for _, attr := range token.Attr {
					if attr.Key == "src" {
						if cacheResource(attr.Val) {
							links[attr.Val] = dumpLink(attr.Val)
						}
					}
				}
			}
		case html.StartTagToken, html.EndTagToken:
			token := z.Token()
			if "script" == token.Data {
				for _, attr := range token.Attr {
					if attr.Key == "src" {
						if cacheResource(attr.Val) {
							links[attr.Val] = dumpLink(attr.Val)
						}
					}
				}
			} else if "link" == token.Data {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						if cacheResource(attr.Val) {
							links[attr.Val] = dumpLink(attr.Val)
						}
					}
				}
			}
		default:
			fmt.Println("Passed a token of other types")
		}
	}
}

// ListenOn ...
func ListenOn(ipPort string) { defaultProxy.ipPort = ipPort }

// UseCache sets the default proxy to use cache cache.
func UseCache(cache cache.Cache) { defaultProxy.cache = cache }

// InterceptGET ...
func InterceptGET() (err error) {
	// Some shit here.
	fmt.Println("Intercepting requests on", defaultProxy.ipPort, "...")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(defaultProxy.ipPort, nil))
	return
}
