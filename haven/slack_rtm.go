package haven

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// MsgChanBufSize is WsClient's message channel size
	MsgChanBufSize = 100

	// ReadTimeout is WsClient's read timeout value
	ReadTimeout  = time.Second * 65
	pingInterval = time.Second * 60
)

// WsClient is websocket client
type WsClient struct {
	conn       *websocket.Conn
	receive    chan []byte
	Receive    <-chan []byte
	disconnect chan error
	Disconnect <-chan error
}

// NewWsClient create new WsClient
func NewWsClient() *WsClient {
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
	c.conn = conn
	go c.readLoop()
	go c.pinger()
	return nil
}

// Close websocket connection
func (c *WsClient) Close() {
	err := c.conn.Close()
	c.conn = nil
	if err != nil {
		logger.Warnf("%v", err)
	}
}

func (c *WsClient) pinger() {
	var seqNo uint = 1
	ticker := time.NewTicker(pingInterval)
	msg := ping{Type: "ping"}
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msg.ID = seqNo
			jsonBytes, err := json.Marshal(msg)
			if err != nil {
				logger.Warnf("ping message error: %v", err)
				continue
			}
			logger.Debug("send ping")
			if err := c.conn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
				logger.Warnf("ping send error: %v", err)
				continue
			}
			seqNo = seqNo + 1
		}
	}
}

func (c *WsClient) readLoop() {
	for {
		err := c.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		if err != nil {
			logger.Warnf("%v", err)
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
