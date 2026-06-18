package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"reqost/internal/httpclient"
)

// SSEService streams Server-Sent Events. Backed by a plain net/http GET with
// `Accept: text/event-stream`, parsing the wire format line-by-line. Frames
// are pushed to the frontend as `sse:event` events (open/event/error/close)
// — same pattern as WSService.
type SSEService struct {
	emitter EventEmitter

	mu    sync.Mutex
	conns map[string]context.CancelFunc // connID → cancel
}

func NewSSEService() *SSEService {
	return &SSEService{conns: make(map[string]context.CancelFunc)}
}

func (s *SSEService) setEmitter(e EventEmitter) { s.emitter = e }

func (s *SSEService) emit(connID, typ, data string) {
	if s.emitter == nil {
		return
	}
	s.emitter.Emit("sse:event", map[string]any{
		"connId": connID,
		"type":   typ,
		"data":   data,
		"ts":     time.Now().UnixMilli(),
	})
}

// Connect dials url, sends GET with SSE Accept header, and streams events.
// Variables are NOT interpolated server-side here — the frontend already
// resolves them (like WSService). headers can carry an Authorization etc.
func (s *SSEService) Connect(connID, url string, headers []httpclient.Header) error {
	if connID == "" {
		return fmt.Errorf("connID required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.conns[connID] = cancel
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		s.drop(connID)
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for _, h := range headers {
		if !h.Enabled || h.Key == "" {
			continue
		}
		req.Header.Add(h.Key, h.Value)
	}

	go func() {
		defer s.drop(connID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			s.emit(connID, "error", err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			s.emit(connID, "error", fmt.Sprintf("HTTP %d", resp.StatusCode))
			return
		}
		s.emit(connID, "open", fmt.Sprintf("HTTP %d", resp.StatusCode))
		s.readLoop(ctx, connID, resp.Body)
	}()
	return nil
}

// readLoop parses the SSE wire format: lines beginning with `data:`,
// `event:`, `id:`, blank line = dispatch.
func (s *SSEService) readLoop(ctx context.Context, connID string, body io.Reader) {
	br := bufio.NewReaderSize(body, 1<<20)
	var eventName, data string
	flush := func() {
		if data == "" && eventName == "" {
			return
		}
		typ := "event"
		if eventName != "" {
			typ = eventName
		}
		s.emit(connID, typ, strings.TrimRight(data, "\n"))
		eventName, data = "", ""
	}
	for {
		select {
		case <-ctx.Done():
			s.emit(connID, "close", "cancelled")
			return
		default:
		}
		line, err := br.ReadString('\n')
		if err != nil {
			flush()
			if err != io.EOF {
				s.emit(connID, "error", err.Error())
			} else {
				s.emit(connID, "close", "EOF")
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // SSE comment
		}
		if i := strings.IndexByte(line, ':'); i > 0 {
			field := line[:i]
			value := strings.TrimPrefix(line[i+1:], " ")
			switch field {
			case "data":
				data += value + "\n"
			case "event":
				eventName = value
			case "id":
				// id is metadata — we surface it inline as a comment-like frame.
				s.emit(connID, "id", value)
			case "retry":
				s.emit(connID, "retry", value)
			}
		}
	}
}

// Close cancels the connection.
func (s *SSEService) Close(connID string) {
	s.mu.Lock()
	cancel := s.conns[connID]
	delete(s.conns, connID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *SSEService) drop(connID string) {
	s.mu.Lock()
	if cancel, ok := s.conns[connID]; ok {
		cancel()
		delete(s.conns, connID)
	}
	s.mu.Unlock()
}
