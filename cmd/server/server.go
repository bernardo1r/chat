package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/bernardo1r/chat/handler"
)

func main() {
	hub := handler.NewHub()
	go hub.Run()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handler.Serve(w, r, hub)
	})

	var (
		listener net.Listener
		err      error
	)
	if len(os.Args) == 1 {
		listener, err = net.Listen("tcp4", ":0")
	} else {
		listener, err = net.Listen("tcp4", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}

	errChan := make(chan error)
	go func() {
		errChan <- http.ServeTLS(listener, nil, "certs/server.crt", "certs/server.key")
	}()

	tick := time.NewTicker(time.Second)
loop:
	for {
		select {
		case <-tick.C:
			tick.Stop()
			handler.TestConn(listener.Addr().String())
		case err = <-errChan:
			break loop
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
