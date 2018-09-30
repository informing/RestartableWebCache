package proxy

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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

func parseLink(z *html.Tokenizer, expected []byte) []byte {
	for key, val, hasAttr := z.TagAttr(); hasAttr == true; key, val, hasAttr = z.TagAttr() {
		if bytes.Equal(key, expected) {
			return val
		}
		fmt.Println(key, val)
	}
	return nil
}

func cacheResource(filepath string, tag int) (cached bool) {
	// check if Resouce URI is relative
	if filepath[0] == '/' && !strings.HasPrefix(filepath, "//") {
		return false
	} else if filepath[0] != '/' && (!strings.HasPrefix(filepath, "http://") || !strings.HasPrefix(filepath, "https://")) {
		return false
	}
	// nreq.Header.Set("Content-Type", contentTypes[tag])
	resp, err := http.Get(filepath)
	if err != nil {
		// Cannot find URI
		return false
	}
	// cache.save(resp, filepath)
	return true
}

func handler(wr http.ResponseWriter, req *http.Request) {
	var resp *http.Response
	var err error
	var nreq *http.Request
	client := &http.Client{}

	nreq, err = http.NewRequest(req.Method, req.RequestURI, req.Body)
	for name, value := range req.Header {
		nreq.Header.Set(name, value[0])
	}
	resp, err = client.Do(nreq)

	if err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	doctype := nreq.Header.Get("Content-Type")
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if strings.HasPrefix(doctype, "text/html") {
		// cache the result

		// parse the html and cache the files
		parse(resp)
	} else if strings.HasPrefix(doctype, "text/javascript") {
		// serve the file specified by its name
	} else if strings.HasPrefix(doctype, "octect/image") {
	} else if strings.HasPrefix(doctype, "text/css") {

	}
	for k, v := range resp.Header {
		wr.Header().Set(k, v[0])
	}
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	resp.Body.Close()

	// PrintHTTP(req, resp)
}

func parse(r io.Reader) {
	depth := 0
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return z.Err()
		case html.TextToken:
			if depth > 0 {
				// emitBytes should copy the []byte it receives,
				// if it doesn't process it immediately.
				emitBytes(z.Text())
			}
		case html.SelfClosingTagToken:
			tn, hasAttr := z.TagName()
			if len(tn) == 3 && bytes.Equal(tn, []byte("img")) {
				link := parseLink("src")
				cacheResource(link)
			}
			if len(tn) == 6 && tn == "script" {
				link := parseLink("src")
				cacheResource(link)
			}
			if len(tn) == 4 && tn == "link" {
				link := parseLink("href")
				cacheResource(link)
			}
		case html.StartTagToken, html.EndTagToken:
			tn, _ := z.TagName()
			/*
				if len(tn) == 1 && tn[0] == 'a' {
					if tt == html.StartTagToken {
						depth++
					} else {
						depth--
					}
				}
			*/
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
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(defaultProxy.ipPort, nil))
	return
}
