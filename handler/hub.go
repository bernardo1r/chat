package handler

import (
	"fmt"
	"strings"
)

type registerReq struct {
	Name string
	Send chan []byte
	Res  chan bool
}

type Hub struct {
	clients    map[string]chan []byte
	register   chan *registerReq
	unregister chan string
	broadcast  chan []byte
}

func (h *Hub) ClientsList() string {
	var sb strings.Builder
	sb.WriteString("[")
	for key := range h.clients {
		sb.WriteString(key + ", ")
	}
	str := strings.TrimSuffix(sb.String(), ", ")
	str += "]"
	return str
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]chan []byte),
		register:   make(chan *registerReq),
		unregister: make(chan string),
		broadcast:  make(chan []byte),
	}
}

func (h *Hub) Register(name string, SendBufferSize int) (chan []byte, error) {
	if SendBufferSize < 0 {
		return nil, fmt.Errorf("negative buffer size")
	}
	if name == "" {
		return nil, fmt.Errorf("empty name")
	}

	req := &registerReq{
		Name: name,
		Send: make(chan []byte, SendBufferSize),
		Res:  make(chan bool),
	}

	h.register <- req
	ok := <-req.Res
	if ok {
		return nil, fmt.Errorf("user already connected")
	}

	return req.Send, nil
}

func (h *Hub) Unregister(name string) {
	h.unregister <- name
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *Hub) Run() {
	for {
		select {
		case req := <-h.register:
			_, ok := h.clients[req.Name]
			req.Res <- ok
			if !ok {
				h.clients[req.Name] = req.Send
			}

		case name := <-h.unregister:
			close(h.clients[name])
			delete(h.clients, name)

		case message := <-h.broadcast:
			for name, send := range h.clients {
				select {
				case send <- message:

				default:
					close(send)
					delete(h.clients, name)
				}
			}
		}
	}
}
