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
	Request      *http.Request
	RequestBody  []byte
	Response     *http.Response
	ResponseBody []byte
}

var port uint
var proxyMatcher *regexp.Regexp
var reqResChan chan *reqResPair
var httpClient *http.Client

func init() {
	flag.UintVar(&port, "port", 8080, "port number listens HTTP")
	flag.Parse()

	proxyMatcher = regexp.MustCompile(`^/proxy/([^/]+)(/.*)$`)
	reqResChan = make(chan *reqResPair)
	httpClient = http.DefaultClient
}

func main() {
	go printer(reqResChan, os.Stdout)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		subReq, reqBody, err := createSubReq(r)
		panicIf(err)

		res, err := httpClient.Do(subReq)
		panicIf(err)

		resBody := make([]byte, res.ContentLength)

		_, err = res.Body.Read(resBody)
		if err != io.EOF {
			panicIf(err)
		}

		for key, values := range res.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.WriteHeader(res.StatusCode)
		w.Write(resBody)

		reqResPair := &reqResPair{
			Request:      subReq,
			RequestBody:  reqBody,
			Response:     res,
			ResponseBody: resBody,
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

func createSubReq(r *http.Request) (*http.Request, []byte, error) {
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
		return nil, []byte{}, err
	}

	subReq.Header = r.Header

	return subReq, body, nil
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
