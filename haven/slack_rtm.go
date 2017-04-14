package haven

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// ReadLimit is max size of received RTM API message. ref: https://api.slack.com/rtm#limits
	ReadLimit = 16 * 1024

	// MsgChanBufSize is WsClient's message channel size
	MsgChanBufSize = 100

	// ReadTimeout is WsClient's read timeout value
	ReadTimeout = time.Second * 65
)

// WsClient is websocket client
type WsClient struct {
	conn       *websocket.Conn
	receive    chan []byte
	Receive    <-chan []byte
	disconnect chan error
	Disconnect <-chan error
}

// NewWsCleint create new WsClient
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

// Connect to slack websocket api and start read loop
func (c *WsClient) Connect(url string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return err
	}
	conn.SetReadLimit(ReadLimit)
	c.conn = conn
	go c.readLoop()
	return nil
}

// Close websocket connection
func (c *WsClient) Close() {
	err := c.conn.Close()
	c.conn = nil
	if err != nil {
		logger.Warningf("%v", err)
	}
}

func (c *WsClient) readLoop() {
	for {
		err := c.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		if err != nil {
			logger.Warningf("%v", err)
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
