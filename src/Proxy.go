package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)
import "golang.org/x/net/html"

type Page struct {
	URI string
	Body  []byte
}

func (p *Page) save() error {
	// TODO: change filename to hash(p) and content to gob.encode(p)
	filename := p.URI + ".txt"
	// 0600 indicates that the file should be created with read-write permissions for the current user only
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{URI: title, Body: body}, nil
}

func PrintHTTP(request *http.Request, response *http.Response) {
	fmt.Printf("%v %v\n", request.Method, request.RequestURI)
	for k, v := range request.Header {
		fmt.Println(k, ":", v)
	}
	fmt.Println("==============================")
	fmt.Printf("HTTP/1.1 %v\n", response.Status)
	for k, v := range response.Header {
		fmt.Println(k, ":", v)
	}
	fmt.Println(response.Body)
	fmt.Println("==============================")
}

//func HandleHTTP() {
//	for {
//		select {
//		case conn := <-connChannel:
//			PrintHTTP(conn)
//		}
//	}
//}
//
//type Proxy struct {
//}
//
//func NewProxy() *Proxy { return &Proxy{} }
//
//func (p *Proxy) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
//
//	//connChannel <- &HttpConnection{r,resp}
//}

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
	// req.Body.Close()

	if err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	doctype := nreq.Header.Get("Content-Type")
	if strings.HasPrefix(doctype, "text/html") {
		// cache the result

		// parse the html and cache the files

	}
	for k, v := range resp.Header {
		wr.Header().Set(k, v[0])
	}
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	resp.Body.Close()

	PrintHTTP(req, resp)
}

func main() {
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
	//proxy := NewProxy()
}