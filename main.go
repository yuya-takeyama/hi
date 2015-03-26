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
			printRequest(w, reqRes.Request, reqRes.RequestBody)
			fmt.Fprint(w, "\n\n")
			printResponse(w, reqRes.Response, reqRes.ResponseBody)
			fmt.Fprint(w, "\n----------\n")
		}
	}
}

func createSubReq(r *http.Request) (*http.Request, []byte, error) {
	body := make([]byte, r.ContentLength)

	_, err := r.Body.Read(body)
	if err != io.EOF {
		panicIf(err)
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

func printRequest(w io.Writer, req *http.Request, body []byte) {
	fmt.Fprintln(w, ansi.Color("Request:", "blue"))

	fmt.Fprintf(w, "%s %s %s\r\n", req.Method, req.URL.RequestURI(), req.Proto)
	req.Header.Write(w)
	io.WriteString(w, "\n")
	w.Write(body)
}

func printResponse(w io.Writer, res *http.Response, body []byte) {
	fmt.Fprintln(w, ansi.Color("Response:", "green"))

	fmt.Fprintf(w, "%s %s\r\n", res.Proto, res.Status)
	res.Header.Write(w)
	io.WriteString(w, "\n")
	w.Write(body)
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
