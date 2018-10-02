package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

func parseLink(z *html.Tokenizer, expectedAttr string) (link string, err error) {
	attr := []byte(expectedAttr)
	for key, val, hasAttr := z.TagAttr(); hasAttr == true; key, val, hasAttr = z.TagAttr() {
		if bytes.Equal(key, attr) {
			fmt.Println("func parseLink: found", key, "with", val)
			return string(val), nil
		}
		fmt.Println(key, val)
	}
	return "", errors.New("cannot locate the resource")
}

func cacheResource(filepath string) (cached bool) {
	// check if Resouce URI is relative
	if filepath[0] == '/' && !strings.HasPrefix(filepath, "//") {
		return false
	} else if filepath[0] != '/' && (!strings.HasPrefix(filepath, "http://") || !strings.HasPrefix(filepath, "https://")) {
		return false
	}
	resp, err := http.Get(filepath)
	if err != nil {
		// Cannot find URI
		return false
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	var testBuffer bytes.Buffer
	testBuffer.Write(body)
	key, _ := url.Parse(filepath)
	defaultProxy.cache.Save(*key, &testBuffer)
	return true
}

func handler(wr http.ResponseWriter, req *http.Request) {
	var resp *http.Response
	var err error
	var nreq *http.Request
	client := &http.Client{}

	fmt.Println("got", req.RequestURI)

	resource, err := url.Parse(req.RequestURI)
	if err != nil {
		nreq, err = http.NewRequest(req.Method, req.RequestURI, req.Body)
		for name, value := range req.Header {
			nreq.Header.Set(name, value[0])
		}
		resp, err = client.Do(nreq)
		req.Body.Close()

		// combined for GET/POST
		if err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		for k, v := range resp.Header {
			wr.Header().Set(k, v[0])
		}
		wr.WriteHeader(resp.StatusCode)
		io.Copy(wr, resp.Body)
		resp.Body.Close()
	} else {
		buf, err := defaultProxy.cache.Get(*resource)

		fmt.Println("got resource")
		if err == cache.ErrResourceNotInCache {
			fmt.Println("not in cache")
			nreq, err = http.NewRequest(req.Method, req.RequestURI, req.Body)
			for name, value := range req.Header {
				nreq.Header.Set(name, value[0])
			}
			resp, err = client.Do(nreq)
			fmt.Println("client done")

			if err != nil {
				http.Error(wr, err.Error(), http.StatusInternalServerError)
				return
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("read all")
				var testBuffer bytes.Buffer
				testBuffer.Write(body)
				go defaultProxy.cache.Save(*resource, &testBuffer)

				for k, v := range resp.Header {
					wr.Header().Set(k, v[0])
				}
				fmt.Println("bef wrote")
				wr.WriteHeader(resp.StatusCode)
				fmt.Println("wrote")
				wr.Write(body)
				resp.Body.Close()
				fmt.Println("sent")
			}
			fmt.Println("parsing...")
			err = parse(resp.Body)
		} else {
			fmt.Println("hits cache")
			body := make([]byte, 0)
			_, err := buf.Write(body)
			if err == nil {
				wr.Write(body)
			} else {
				fmt.Println(err)
			}
		}
	}

	// PrintHTTP(req, resp)
}

func parse(r io.Reader) error {
	// depth := 0
	z := html.NewTokenizer(r)
	for {
		fmt.Println("forever")
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return z.Err()
		/*
			case html.TextToken:
				if depth > 0 {
					// emitBytes should copy the []byte it receives,
					// if it doesn't process it immediately.
					emitBytes(z.Text())
				}
		*/
		case html.SelfClosingTagToken:
			tn, hasAttr := z.TagName()
			if !hasAttr {
				continue
			} else {
				fmt.Println("found", tn)
			}
			if len(tn) == 3 && bytes.Equal(tn, []byte("img")) {
				link, err := parseLink(z, "src")
				if err == nil {
					cacheResource(link)
				}
			}
			if len(tn) == 6 && bytes.Equal(tn, []byte("script")) {
				link, err := parseLink(z, "src")
				if err == nil {
					cacheResource(link)
				}
			}
			if len(tn) == 4 && bytes.Equal(tn, []byte("link")) {
				link, err := parseLink(z, "href")
				if err == nil {
					cacheResource(link)
				}
			}
		case html.StartTagToken, html.EndTagToken:
			tn, _ := z.TagName()
			fmt.Println("found", tn)
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
