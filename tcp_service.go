package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// TCPService manages raw TCP / TLS / UDP socket connections — an in-app netcat.
// Mirrors WSService: connections are keyed by a frontend connId; inbound bytes
// and state changes are pushed as "tcp:event" Wails events. Binary-safe: data
// frames carry base64 (b64:true), status frames carry plain text (b64:false).
type TCPService struct {
	emitter EventEmitter

	mu    sync.Mutex
	conns map[string]*tcpConn
}

type tcpConn struct {
	conn   net.Conn
	cancel context.CancelFunc
}

func NewTCPService() *TCPService { return &TCPService{conns: make(map[string]*tcpConn)} }

func (s *TCPService) setEmitter(e EventEmitter) { s.emitter = e }

func (s *TCPService) emit(connID, typ, dir, data string, b64 bool) {
	if s.emitter != nil {
		s.emitter.Emit("tcp:event", map[string]any{
			"connId": connID, "type": typ, "dir": dir, "data": data, "b64": b64,
			"ts": time.Now().UnixMilli(),
		})
	}
}

// Connect dials rawURL. The scheme picks the transport: tcp:// (plain),
// tls:// (TLS over TCP), udp:// (datagram). A bare host:port means tcp://.
func (s *TCPService) Connect(connID, rawURL string) error {
	network, address, useTLS, err := parseTCPTarget(rawURL)
	if err != nil {
		s.emit(connID, "error", "", err.Error(), false)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	var conn net.Conn
	if useTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, network, address)
	}
	if err != nil {
		cancel()
		s.emit(connID, "error", "", err.Error(), false)
		return err
	}

	s.mu.Lock()
	s.conns[connID] = &tcpConn{conn: conn, cancel: cancel}
	s.mu.Unlock()

	label := strings.ToUpper(network)
	if useTLS {
		label = "TLS"
	}
	s.emit(connID, "open", "", fmt.Sprintf("Connected to %s (%s)", address, label), false)

	go s.readLoop(connID, conn)
	return nil
}

func (s *TCPService) readLoop(connID string, conn net.Conn) {
	buf := make([]byte, 64*1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			s.emit(connID, "data", "in", base64.StdEncoding.EncodeToString(buf[:n]), true)
		}
		if err != nil {
			s.emit(connID, "close", "", closeReason(err), false)
			s.drop(connID)
			return
		}
	}
}

// Send writes data on the connection. mode: "text" (as-is), "line" (append \n),
// "crlf" (append \r\n), "hex" (decode whitespace/colon-separated hex to bytes).
func (s *TCPService) Send(connID, data, mode string) error {
	s.mu.Lock()
	c := s.conns[connID]
	s.mu.Unlock()
	if c == nil {
		return nil
	}
	payload, err := encodeSend(data, mode)
	if err != nil {
		s.emit(connID, "error", "", err.Error(), false)
		return err
	}
	if _, err := c.conn.Write(payload); err != nil {
		s.emit(connID, "error", "", err.Error(), false)
		return err
	}
	s.emit(connID, "data", "out", base64.StdEncoding.EncodeToString(payload), true)
	return nil
}

func (s *TCPService) Close(connID string) {
	s.mu.Lock()
	c := s.conns[connID]
	delete(s.conns, connID)
	s.mu.Unlock()
	if c != nil {
		c.cancel()
		_ = c.conn.Close()
	}
}

func (s *TCPService) drop(connID string) {
	s.mu.Lock()
	c := s.conns[connID]
	delete(s.conns, connID)
	s.mu.Unlock()
	if c != nil {
		c.cancel()
	}
}

// closeReason turns a read error into a human-readable status line. A plain EOF
// is the remote peer closing cleanly (normal for HTTP/idle-timeout/one-shot
// protocols), not an error condition.
func closeReason(err error) string {
	switch {
	case errors.Is(err, io.EOF):
		return "connection closed by remote (EOF)"
	case errors.Is(err, net.ErrClosed):
		return "disconnected"
	default:
		return err.Error()
	}
}

func parseTCPTarget(raw string) (network, address string, useTLS bool, err error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "tls://"):
		address = strings.TrimPrefix(raw, "tls://")
		network, useTLS = "tcp", true
	case strings.HasPrefix(raw, "tcp://"):
		address = strings.TrimPrefix(raw, "tcp://")
		network = "tcp"
	case strings.HasPrefix(raw, "udp://"):
		address = strings.TrimPrefix(raw, "udp://")
		network = "udp"
	default:
		address, network = raw, "tcp" // bare host:port
	}
	address = strings.TrimRight(address, "/")
	if address == "" {
		return "", "", false, fmt.Errorf("empty address")
	}
	return network, address, useTLS, nil
}

func encodeSend(data, mode string) ([]byte, error) {
	switch mode {
	case "hex":
		clean := strings.Map(func(r rune) rune {
			switch r {
			case ' ', '\n', '\r', '\t', ':', '-':
				return -1
			}
			return r
		}, data)
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("invalid hex: %w", err)
		}
		return b, nil
	case "line":
		return []byte(data + "\n"), nil
	case "crlf":
		return []byte(data + "\r\n"), nil
	default: // "text"
		return []byte(data), nil
	}
}
