package client

import (
	"fmt"
	"sync"
	"time"
	"wstest/common"

	"github.com/gorilla/websocket"
)

const DefaultReceiveBufferSize = 128

type Client struct {
	conn     *websocket.Conn
	in       chan *common.Message
	out      chan *sendReq
	err      error
	errMutex sync.Mutex
}

type sendReq struct {
	Message []byte
	Res     chan error
}

func newClient(conn *websocket.Conn, receiveBufferSize int) *Client {
	return &Client{
		conn: conn,
		in:   make(chan *common.Message, receiveBufferSize),
		out:  make(chan *sendReq),
		err:  nil,
	}
}

func (c *Client) Send(message []byte) error {
	if c.err != nil {
		return c.err
	}

	req := &sendReq{
		Message: message,
		Res:     make(chan error),
	}
	c.out <- req
	err := <-req.Res
	c.writeErr(err)

	return c.err
}

func (c *Client) Close() {
	c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	c.conn.Close()
}

func (c *Client) Receive() (*common.Message, error) {
	//buffered channel needed in order to respond pings
	if c.err != nil {
		return nil, c.err
	}

	message, ok := <-c.in
	if !ok {
		//channel closed but err == nil
		for c.err == nil {
		}
		return nil, c.err
	}
	return message, nil
}

func (c *Client) writeErr(err error) {
	c.errMutex.Lock()
	defer c.errMutex.Unlock()
	if c.err == nil {
		c.err = err
	}
}

func (c *Client) read() {
	defer c.conn.Close()
	defer close(c.in)

	for {
		t, message, err := c.conn.ReadMessage()
		if err != nil {
			c.writeErr(err)
			return
		}
		if t != websocket.BinaryMessage {
			c.writeErr(fmt.Errorf("wrong message type"))
			return
		}

		decodedMessage, err := common.DecodeMessage(message)
		if err != nil {
			c.writeErr(err)
			return
		}

		select {
		case c.in <- &decodedMessage:
		default:
			c.writeErr(fmt.Errorf("receive buffer full"))
			return
		}
	}
}

func (c *Client) write() {
	defer c.conn.Close()

	for {
		req := <-c.out
		err := c.conn.WriteMessage(websocket.TextMessage, []byte(req.Message))
		if err != nil {
			c.conn.Close()
		}
		req.Res <- err
	}

}

func nameSend(conn *websocket.Conn, name string) error {
	conn.SetWriteDeadline(time.Now().Add(common.WriteWait))
	err := conn.WriteMessage(websocket.TextMessage, []byte(name))
	if err != nil {
		return err
	}

	return err
}

func nameResponse(conn *websocket.Conn) error {
	conn.SetReadDeadline(time.Now().Add(common.NameWait))
	t, message, err := conn.ReadMessage()
	if t != websocket.TextMessage {
		return fmt.Errorf("invalid message type from server")
	}
	if err != nil {
		return err
	}
	if string(message) != "OK" {
		return fmt.Errorf(string(message))
	}

	return nil
}

func handshake(conn *websocket.Conn, name string) error {
	err := nameSend(conn, name)
	if err != nil {
		return err
	}

	err = nameResponse(conn)
	if err != nil {
		return err
	}

	return nil
}

func Upgrade(conn *websocket.Conn, name string) (*Client, error) {

	err := handshake(conn, name)
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})
	client := newClient(conn, DefaultReceiveBufferSize)
	go client.read()
	go client.write()

	return client, nil
}
