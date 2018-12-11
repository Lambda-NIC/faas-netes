// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
	"strings"

	"github.com/gorilla/mux"
	"github.com/Lambda-NIC/faas/gateway/requests"
	"go.etcd.io/etcd/client"
)

// MakeProxy creates a proxy for HTTP web requests which can be routed to a function.
func MakeProxy(functionNamespace string, keysAPI client.KeysAPI,
							 timeout time.Duration) http.HandlerFunc {
	proxyClient := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 1 * time.Second,
			}).DialContext,
			// MaxIdleConns:          1,
			// DisableKeepAlives:     false,
			IdleConnTimeout:       120 * time.Millisecond,
			ExpectContinueTimeout: 1500 * time.Millisecond,
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body != nil {
			defer r.Body.Close()
		}

		switch r.Method {
		case http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodGet:

			vars := mux.Vars(r)
			service := vars["name"]

			stamp := strconv.FormatInt(time.Now().Unix(), 10)

			defer func(when time.Time) {
				seconds := time.Since(when).Seconds()
				log.Printf("[%s] took %f seconds\n", stamp, seconds)
			}(time.Now())

			forwardReq := requests.NewForwardRequest(r.Method, *r.URL)

			url := forwardReq.ToURL(fmt.Sprintf("%s.%s", service, functionNamespace), watchdogPort)

			if strings.Contains(service, "lambdaNIC") {
				// TODO: Send to smartNIC and wait or let it send response back?
				//clientHeader := w.Header()
				//copyHeaders(&clientHeader, &response.Header)
				//writeHead(service, http.StatusOK, w)
				//io.Copy(w, "Hello")
				log.Println("Need a proxy for SmartNICs")
				return
			}
			request, _ := http.NewRequest(r.Method, url, r.Body)

			copyHeaders(&request.Header, &r.Header)

			defer request.Body.Close()

			response, err := proxyClient.Do(request)

			if err != nil {
				log.Println(err.Error())
				writeHead(service, http.StatusInternalServerError, w)
				buf := bytes.NewBufferString("Can't reach service: " + service)
				w.Write(buf.Bytes())
				return
			}

			clientHeader := w.Header()
			copyHeaders(&clientHeader, &response.Header)

			writeHead(service, http.StatusOK, w)
			io.Copy(w, response.Body)
		}
	}
}

func writeHead(service string, code int, w http.ResponseWriter) {
	w.WriteHeader(code)
}

func copyHeaders(destination *http.Header, source *http.Header) {
	for k, v := range *source {
		vClone := make([]string, len(v))
		copy(vClone, v)
		(*destination)[k] = vClone
	}
}
