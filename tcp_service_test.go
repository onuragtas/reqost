package main

import (
	"encoding/base64"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type captureEmitter struct {
	mu     sync.Mutex
	events []map[string]any
}

func (e *captureEmitter) Emit(_ string, data ...any) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(data) > 0 {
		if m, ok := data[0].(map[string]any); ok {
			e.events = append(e.events, m)
		}
	}
	return true
}
func (e *captureEmitter) findData(dir string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, m := range e.events {
		if m["type"] == "data" && m["dir"] == dir {
			b, _ := base64.StdEncoding.DecodeString(m["data"].(string))
			return string(b), true
		}
	}
	return "", false
}

func TestTCPEchoRoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		io.Copy(c, c) // echo
		c.Close()
	}()

	em := &captureEmitter{}
	s := NewTCPService()
	s.setEmitter(em)

	if err := s.Connect("c1", "tcp://"+ln.Addr().String()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer s.Close("c1")
	if err := s.Send("c1", "hello", "line"); err != nil {
		t.Fatalf("send: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got, ok := em.findData("in"); ok {
			if got != "hello\n" {
				t.Fatalf("echo = %q, want \"hello\\n\"", got)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no inbound data received within 2s")
}

func TestParseTCPTarget(t *testing.T) {
	cases := []struct {
		in      string
		net     string
		addr    string
		useTLS  bool
		wantErr bool
	}{
		{"tcp://localhost:6379", "tcp", "localhost:6379", false, false},
		{"tls://example.com:443", "tcp", "example.com:443", true, false},
		{"udp://1.1.1.1:53", "udp", "1.1.1.1:53", false, false},
		{"localhost:25", "tcp", "localhost:25", false, false},
		{"  tcp://h:1/  ", "tcp", "h:1", false, false},
		{"", "", "", false, true},
	}
	for _, c := range cases {
		n, a, tlsf, err := parseTCPTarget(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%q err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if n != c.net || a != c.addr || tlsf != c.useTLS {
			t.Errorf("%q → (%q,%q,%v), want (%q,%q,%v)", c.in, n, a, tlsf, c.net, c.addr, c.useTLS)
		}
	}
}

func TestEncodeSend(t *testing.T) {
	cases := []struct {
		data, mode string
		want       []byte
		wantErr    bool
	}{
		{"hi", "text", []byte("hi"), false},
		{"hi", "line", []byte("hi\n"), false},
		{"hi", "crlf", []byte("hi\r\n"), false},
		{"48 65:6c-6c6f", "hex", []byte("Hello"), false},
		{"zz", "hex", nil, true},
	}
	for _, c := range cases {
		got, err := encodeSend(c.data, c.mode)
		if (err != nil) != c.wantErr {
			t.Errorf("%q/%s err=%v wantErr=%v", c.data, c.mode, err, c.wantErr)
			continue
		}
		if err == nil && string(got) != string(c.want) {
			t.Errorf("%q/%s = %q, want %q", c.data, c.mode, got, c.want)
		}
	}
}
