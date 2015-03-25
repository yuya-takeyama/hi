package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"

	"github.com/mgutz/ansi"
)

type reqResPair struct {
	Request     *http.Request
	RequestBody []byte
	Response    *http.Response
}

var port uint
var proxyMatcher *regexp.Regexp
var reqResChan chan *reqResPair

func init() {
	flag.UintVar(&port, "port", 8080, "port number listens HTTP")
	flag.Parse()

	proxyMatcher = regexp.MustCompile(`^/proxy/([^/]+)(/.*)$`)
	reqResChan = make(chan *reqResPair)
}

func main() {
	go printer(reqResChan, os.Stdout)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)

		_, err := r.Body.Read(body)
		if err != nil && err != io.EOF {
			fmt.Println(err)
		}

		re := proxyMatcher.FindAllStringSubmatch(r.RequestURI, 2)[0]

		host := re[1]
		uri := re[2]

		subReq, err := http.NewRequest(r.Method, fmt.Sprintf("http://%s%s", host, uri), bytes.NewBuffer(body))
		if err != nil {
			fmt.Println(err)
		}
		subReq.Header = r.Header

		client := http.DefaultClient
		res, err := client.Do(subReq)

		reqResPair := &reqResPair{
			Request:     subReq,
			RequestBody: body,
			Response:    res,
		}

		reqResChan <- reqResPair
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func printer(rrChan chan *reqResPair, w io.Writer) {
	for {
		select {
		case reqRes := <-rrChan:
			fmt.Println(ansi.Color("Request:", "blue"))
			req := reqRes.Request
			res := reqRes.Response

			fmt.Printf("%s %s %s\r\n", req.Method, req.URL.Path, req.Proto)
			req.Header.Write(w)
			w.Write(reqRes.RequestBody)

			fmt.Print("\r\n\r\n")

			fmt.Println(ansi.Color("Response:", "green"))

			fmt.Printf("%s %s\r\n", res.Proto, res.Status)
			res.Header.Write(w)
			body := make([]byte, res.ContentLength)

			_, err := res.Body.Read(body)
			if err != nil && err != io.EOF {
				fmt.Println(err)
			}
			w.Write(body)

			fmt.Println("\n----------\n")
		}
	}
}
