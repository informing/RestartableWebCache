package proxy

import (
	"bytes"
	"errors"
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

func extractLinkFromElement(z *html.Tokenizer, elementTag string) (link string, err error) {
	attr := []byte(elementTag)
	for key, val, hasAttr := z.TagAttr(); hasAttr == true; key, val, hasAttr = z.TagAttr() {
		if bytes.Equal(key, attr) {
			fmt.Println("func extractLinkFromElement: found", key, "with", val)
			return string(val), nil
		}
		fmt.Println(key, val)
	}
	return "", errors.New("cannot locate the resource")
}

func cacheResource(resourceLink string, h http.Header) (cached bool) {
	// check if Resouce URI is relative
	if resourceLink[0] == '/' && !strings.HasPrefix(resourceLink, "//") {
		return false
	} else if resourceLink[0] != '/' && (!strings.HasPrefix(resourceLink, "http://") || !strings.HasPrefix(resourceLink, "https://")) {
		return false
	}
	response, err := http.Get(resourceLink)
	if err != nil {
		// Cannot find URI
		return false
	}
	responseBodyData, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	var responseBuffer bytes.Buffer
	responseBuffer.Write(responseBodyData)
	resourceURL, _ := url.Parse(resourceLink)
	fmt.Println("Saving", resourceLink, "to cache")
	defaultProxy.cache.SaveWithHeaders(*resourceURL, &responseBuffer, h)
	return true
}

func hash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return "[" + strconv.Itoa(int(h.Sum32())) + "]"
}

func handler(proxyWriter http.ResponseWriter, clientRequest *http.Request) {
	var err error
	var serverResponse *http.Response
	var proxyRequest *http.Request
	client := &http.Client{}

	resourceURL, _ := url.Parse(clientRequest.RequestURI)
	hashedLink := hash(clientRequest.RequestURI)
	fmt.Println("Client requested", clientRequest.Method, clientRequest.RequestURI, hashedLink)

	if strings.HasPrefix(clientRequest.RequestURI, "http://") && clientRequest.Method == "GET" {
		// We only handle http GET requests
		fmt.Println("Trying to fetch resource from cache.Get", hashedLink)
		cachedResponse, originalHeaders, err := defaultProxy.cache.GetWithHeaders(*resourceURL)
		if err == cache.ErrResourceNotInCache {
			fmt.Println("The requested resource is not in cache", hashedLink)
			proxyRequest, err = http.NewRequest(clientRequest.Method, clientRequest.RequestURI, clientRequest.Body)
			for name, value := range clientRequest.Header {
				proxyRequest.Header.Set(name, value[0])
			}
			serverResponse, err = client.Do(proxyRequest)
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
					proxyWriter.Header().Set(k, v[0])
				}
				proxyWriter.WriteHeader(serverResponse.StatusCode)

				var responseBuffer bytes.Buffer
				multiWriter := io.MultiWriter(proxyWriter, &responseBuffer)
				multiWriter.Write(responseBodyData)
				defer serverResponse.Body.Close()

				// responseBuffer.Write(responseBodyData)
				fmt.Println("Calling cache.Save to cache the server response")
				defaultProxy.cache.SaveWithHeaders(*resourceURL, bytes.NewBuffer(responseBuffer.Bytes()), serverResponse.Header)

				if strings.HasPrefix(serverResponse.Header.Get("Content-Type"), "text/html") {
					fmt.Println("Parsing the response body to find more resources to cache")
					err = ParseResponseBody(&responseBuffer, serverResponse.Header)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		} else {
			fmt.Println("Got the requested resource from cache")
			fmt.Println("Serving content to browser...")

			// Make a temporary copy of this cache resource.  We do not
			// want to drain the actual buffer in the cache.
			var tmp bytes.Buffer
			if _, err := tmp.Write(cachedResponse.Bytes()); err != nil {
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
	} else {
		fmt.Println("Cannot parse the provided URI, will simply serve w/o caching")
		proxyRequest, err = http.NewRequest(clientRequest.Method, clientRequest.RequestURI, clientRequest.Body)
		for name, value := range clientRequest.Header {
			proxyRequest.Header.Set(name, value[0])
		}
		serverResponse, err = client.Do(proxyRequest)
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
}

// ParseResponseBody parses the given reader
func ParseResponseBody(r io.Reader, h http.Header) error {
	// depth := 0
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		fmt.Println("Fetching next token", tt)
		switch tt {
		case html.ErrorToken:
			// Ultimately we will get to this point (EOF)
			return z.Err()
		case html.TextToken:
			continue
		case html.SelfClosingTagToken:
			tn, hasAttr := z.TagName()
			if !hasAttr {
				continue
			} else {
				fmt.Println("Found a Self-Closing Token", tn)
			}
			if len(tn) == 3 && bytes.Equal(tn, []byte("img")) {
				link, err := extractLinkFromElement(z, "src")
				if err == nil {
					fmt.Println("<img> contains 'src'")
					cacheResource(link, h)
				}
			}
			if len(tn) == 6 && bytes.Equal(tn, []byte("script")) {
				link, err := extractLinkFromElement(z, "src")
				if err == nil {
					fmt.Println("<script> contains 'src'")
					cacheResource(link, h)
				}
			}
			if len(tn) == 4 && bytes.Equal(tn, []byte("link")) {
				link, err := extractLinkFromElement(z, "href")
				if err == nil {
					fmt.Println("<link> contains 'href'")
					cacheResource(link, h)
				}
			}
		case html.StartTagToken, html.EndTagToken:
			tn, _ := z.TagName()
			fmt.Println("Passed a Normal Token", tn)
		/*
			if len(tn) == 1 && tn[0] == 'a' {
				if tt == html.StartTagToken {
					depth++
				} else {
					depth--
				}
			}
		*/
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
