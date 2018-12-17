// Copyright (c) Sean Choi 2018. All rights reserved.

package handlers

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
)

const udpPacketSize = 10

func sendReceiveLambdaNic(addrStr string, port int, data string) string {
	var wg sync.WaitGroup
	var inbound string
	udpAddr := net.UDPAddr{IP: net.ParseIP(addrStr), Port: port}

	conn, err := net.ListenUDP("udp4", &udpAddr)
	if err != nil {
		log.Printf("Error: UDP conn error: %v", err)
		return ""
	}
	defer conn.Close()

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := conn.WriteToUDP([]byte(data), &udpAddr)
		if err != nil {
			log.Printf("Error: UDP write error: %v", err)
		} else {
			log.Printf("Wrote: %s to %s:%d", data, addrStr, port)
		}
	}()

	go func() {
		defer wg.Done()
		b := make([]byte, udpPacketSize)
		for {
			n, _, err := conn.ReadFromUDP(b)
			if err != nil {
				log.Printf("Error: UDP read error: %v", err)
				continue
			}
			b2 := make([]byte, udpPacketSize)
			copy(b2, b)
			inbound = string(b2[:n])
			return
		}
	}()
	wg.Wait()
	return inbound
}

func generateResponse(req *http.Request, body string) *http.Response {
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
