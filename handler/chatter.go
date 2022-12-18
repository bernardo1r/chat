package handler

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"wstest/common"

	"github.com/gorilla/websocket"
)

type client struct {
	name string
	conn *websocket.Conn
	send chan []byte
}

const defaultSendBufferSize = 128

var upgrader = websocket.Upgrader{}

const testConnMessage = "testing conn"

func isTestConnError(err error) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		err, _ := err.(*websocket.CloseError)
		if err.Text == testConnMessage {
			return true
		}
	}

	return false
}

func TestConn(addr string) {
	certPool := x509.NewCertPool()
	pem, err := os.ReadFile("certs/server.crt")
	if err != nil {
		return
	}
	certPool.AppendCertsFromPEM(pem)
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			opts := x509.VerifyOptions{
				Roots: certPool,
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	}
	conn, _, err := dialer.Dial("wss://"+addr, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, testConnMessage))
	if err != nil {
		return
	}

	fmt.Println("Serving in address", addr)
}

func getName(conn *websocket.Conn) (string, error) {
	conn.SetReadDeadline(time.Now().Add(common.NameWait))
	t, message, err := conn.ReadMessage()
	if err != nil {
		return "", err
	}
	if t != websocket.TextMessage {
		return "", fmt.Errorf("invalid message type from client")
	}

	return string(message), nil
}

func (c *client) diconnect(hub *Hub) {
	defer hub.Unregister(c.name)

	msg := common.Message{
		Type: common.DisconnectedMessage,
		Name: c.name,
	}
	msgEncoded, err := msg.EncodeMessage()
	if err != nil {
		return
	}

	hub.Broadcast(msgEncoded)
}

func (c *client) connect(hub *Hub) error {
	msg := common.Message{
		Type: common.ConnectedMessage,
		Name: c.name,
	}
	msgEncoded, err := msg.EncodeMessage()
	if err != nil {
		return err
	}

	hub.Broadcast(msgEncoded)
	return nil
}

func (c *client) sendUsersList(hub *Hub) error {
	msg := common.Message{
		Type:    common.ListMessage,
		Message: hub.ClientsList(),
	}
	msgEncoded, err := msg.EncodeMessage()
	if err != nil {
		return err
	}
	c.send <- msgEncoded
	return nil
}

func (c *client) read(hub *Hub) {
	defer c.conn.Close()
	defer c.diconnect(hub)

	c.conn.SetReadDeadline(time.Now().Add(common.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(common.PongWait))
		return nil
	})
	for {
		t, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Print(err)
			return
		}
		if t != websocket.TextMessage {
			log.Print("received binary message")
			return
		}
		if len(message) == 0 {
			continue
		}
		if string(message) == "/list" {
			err = c.sendUsersList(hub)
			if err != nil {
				log.Print(err)
				return
			}
			continue
		}
		msg := common.Message{
			Type:    common.TextMessage,
			Name:    c.name,
			Message: string(message),
		}
		msgEncoded, err := msg.EncodeMessage()
		if err != nil {
			log.Print(err)
			return
		}
		hub.Broadcast(msgEncoded)
	}
}

func (c *client) write(hub *Hub) {
	defer c.conn.Close()
	ticker := time.NewTicker(common.PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(common.WriteWait))
			if !ok {
				return
			}

			writer, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				log.Print("write nextWriter", err)
				return
			}
			writer.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				writer.Write(<-c.send)
			}

			err = writer.Close()
			if err != nil {
				log.Print("write closing", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(common.WriteWait))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Print("write ping", err)
				return
			}
		}
	}
}

func nameResponse(conn *websocket.Conn, responseErr error) error {
	conn.SetWriteDeadline(time.Now().Add(common.WriteWait))
	var err error
	if responseErr != nil {
		err = conn.WriteMessage(websocket.TextMessage, []byte(responseErr.Error()))
	} else {
		err = conn.WriteMessage(websocket.TextMessage, []byte("OK"))
	}
	return err
}

func Serve(writer http.ResponseWriter, req *http.Request, hub *Hub) {
	conn, err := upgrader.Upgrade(writer, req, nil)
	if err != nil {
		log.Print(err)
		return
	}

	name, err := getName(conn)
	if err != nil {
		if isTestConnError(err) {
			return
		}
		log.Print(err)
		nameResponse(conn, err)
		return
	}

	send, err := hub.Register(name, defaultSendBufferSize)
	if err != nil {
		log.Printf("client %s: %s\n", name, err)
		nameResponse(conn, err)
		return
	}
	nameResponse(conn, nil)

	client := &client{
		name: name,
		conn: conn,
		send: send,
	}

	err = client.connect(hub)
	if err != nil {
		hub.Unregister(client.name)
		client.conn.Close()
		return
	}
	go client.read(hub)
	go client.write(hub)
}
