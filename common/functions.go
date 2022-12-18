package common

import (
	"bytes"
	"encoding/gob"
)

type MessageType int

const (
	ConnectedMessage MessageType = iota
	DisconnectedMessage
	TextMessage
	TimeoutMessage
	ListMessage
)

type Message struct {
	Type    MessageType
	Name    string
	Message string
}

func (msg *Message) EncodeMessage() ([]byte, error) {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(msg)

	return buffer.Bytes(), err
}

func DecodeMessage(stream []byte) (Message, error) {
	buffer := bytes.NewBuffer(stream)
	dec := gob.NewDecoder(buffer)
	var msg Message
	err := dec.Decode(&msg)

	return msg, err
}
