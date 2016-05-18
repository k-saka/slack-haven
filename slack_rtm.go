package main

import (
	"github.com/gorilla/websocket"
	"log"
	"time"
)

const (
	// Slack message max size at RTM API. ref: https://api.slack.com/rtm#limits
	MsgBufSize = 1024 * 16

	// WsClient message channel buffer size
	MsgChanBufSize = 100

	// Client read time out
	ReadTimeout = time.Second * 65
)

// Slack RTM API client
type WsClient struct {
	conn       *websocket.Conn
	receive    chan []byte
	Receive    <-chan []byte
	disconnect chan error
	Disconnect <-chan error
}

// Create a WsClient
func NewWsCleint() *WsClient {
	receive := make(chan []byte, MsgChanBufSize)
	disconnect := make(chan error)
	return &WsClient{
		receive:    receive,
		Receive:    receive,
		disconnect: disconnect,
		Disconnect: disconnect,
	}
}

// Connect slack websocket api and start read loop
func (c *WsClient) Connect(url string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	go c.readLoop()
	return nil
}

// Close connection
func (c *WsClient) Close() {
	err := c.conn.Close()
	c.conn = nil
	if err != nil {
		log.Printf("%v\n", err)
	}
}

// Read message loop
func (c *WsClient) readLoop() {
	for {
		err := c.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		if err != nil {
			log.Printf("%v\n", err)
		}
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.Close()
			c.disconnect <- err
			return
		}
		c.receive <- msg
	}
}
