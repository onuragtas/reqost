package main

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"

	"reqost/internal/httpclient"
)

// WSService manages live WebSocket connections. Each connection is identified by
// a frontend-supplied connId; messages and state changes are pushed to the UI
// as "ws:event" Wails events carrying that connId.
type WSService struct {
	emitter EventEmitter

	mu    sync.Mutex
	conns map[string]*wsConn
}

type wsConn struct {
	c      *websocket.Conn
	cancel context.CancelFunc
}

func NewWSService() *WSService {
	return &WSService{conns: make(map[string]*wsConn)}
}

func (s *WSService) SetEmitter(e EventEmitter) { s.emitter = e }

func (s *WSService) emit(connID, typ, dir, data string) {
	if s.emitter != nil {
		s.emitter.Emit("ws:event", map[string]any{
			"connId": connID, "type": typ, "dir": dir, "data": data,
			"ts": time.Now().UnixMilli(),
		})
	}
}

// Connect dials url and starts pumping incoming frames as events. Variables are
// interpolated into the URL and header values by the caller already; headers
// here are sent as-is.
func (s *WSService) Connect(connID, url string, headers []httpclient.Header) error {
	ctx, cancel := context.WithCancel(context.Background())

	opts := &websocket.DialOptions{}
	if len(headers) > 0 {
		opts.HTTPHeader = map[string][]string{}
		for _, h := range headers {
			if h.Enabled && h.Key != "" {
				opts.HTTPHeader[h.Key] = append(opts.HTTPHeader[h.Key], h.Value)
			}
		}
	}

	c, _, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		cancel()
		s.emit(connID, "error", "", err.Error())
		return err
	}
	c.SetReadLimit(32 << 20)

	s.mu.Lock()
	s.conns[connID] = &wsConn{c: c, cancel: cancel}
	s.mu.Unlock()
	s.emit(connID, "open", "", url)

	go s.readLoop(ctx, connID, c)
	return nil
}

func (s *WSService) readLoop(ctx context.Context, connID string, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			s.emit(connID, "close", "", err.Error())
			s.drop(connID)
			return
		}
		s.emit(connID, "message", "in", string(data))
	}
}

// Send writes a text frame on the named connection.
func (s *WSService) Send(connID, data string) error {
	s.mu.Lock()
	conn := s.conns[connID]
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.c.Write(ctx, websocket.MessageText, []byte(data)); err != nil {
		s.emit(connID, "error", "", err.Error())
		return err
	}
	s.emit(connID, "message", "out", data)
	return nil
}

// Close terminates a connection.
func (s *WSService) Close(connID string) {
	s.mu.Lock()
	conn := s.conns[connID]
	delete(s.conns, connID)
	s.mu.Unlock()
	if conn != nil {
		conn.cancel()
		_ = conn.c.Close(websocket.StatusNormalClosure, "client closed")
	}
}

func (s *WSService) drop(connID string) {
	s.mu.Lock()
	conn := s.conns[connID]
	delete(s.conns, connID)
	s.mu.Unlock()
	if conn != nil {
		conn.cancel()
	}
}
