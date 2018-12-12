// Copyright (c) Sean Choi 2018. All rights reserved.

package handlers

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

func generateResponse(req *http.Request) *http.Response {
	body := "Hello world"
	t := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
		Request:       req,
		Header:        make(http.Header, 0),
	}
	return t
}
