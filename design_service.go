package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

func unmarshalYAML(data []byte, out any) error { return yaml.Unmarshal(data, out) }

// DesignService backs the Design left-rail mode: store an OpenAPI / Swagger
// spec on disk, run a local mock server that replies with response examples
// from that spec, and let the frontend reload the spec on every save.
//
// The mock server is dumb on purpose — it matches incoming requests by method
// + path template and returns the first example from the operation. No request
// validation, no scenario branching. This is the same MVP semantics Postman's
// Mocks offer to free users.
type DesignService struct {
	mu      sync.Mutex
	spec    map[string]any
	specRaw string
	path    string

	srvMu  sync.Mutex
	srv    *http.Server
	srvErr string
	port   int
}

func NewDesignService() *DesignService {
	s := &DesignService{}
	if cacheDir, err := os.UserCacheDir(); err == nil {
		s.path = filepath.Join(cacheDir, "reqost", "design.yaml")
	}
	if data, err := os.ReadFile(s.path); err == nil {
		s.specRaw = string(data)
		_ = s.parse()
	}
	return s
}

// LoadSpec returns the saved spec text (yaml or json).
func (s *DesignService) LoadSpec() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.specRaw
}

// SaveSpec writes the spec to disk and re-parses it. The mock server, if
// running, sees the new spec on the next request.
func (s *DesignService) SaveSpec(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return fmt.Errorf("no cache dir")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.path, []byte(content), 0o644); err != nil {
		return err
	}
	s.specRaw = content
	return s.parse()
}

func (s *DesignService) parse() error {
	s.spec = nil
	if strings.TrimSpace(s.specRaw) == "" {
		return nil
	}
	// json.Decoder is permissive enough for JSON — full YAML parsing lives
	// in internal/openapi for imports; here we only need the path map.
	if strings.HasPrefix(strings.TrimSpace(s.specRaw), "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(s.specRaw), &m); err != nil {
			return err
		}
		s.spec = m
	} else {
		// Punt YAML to internal/openapi which already pulls in yaml.v3.
		var m map[string]any
		if err := unmarshalYAML([]byte(s.specRaw), &m); err != nil {
			return err
		}
		s.spec = m
	}
	return nil
}

// MockStatus reports the running mock server's port and last error, if any.
type MockStatus struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Error   string `json:"error"`
}

func (s *DesignService) MockStatus() MockStatus {
	s.srvMu.Lock()
	defer s.srvMu.Unlock()
	return MockStatus{Running: s.srv != nil, Port: s.port, Error: s.srvErr}
}

// StartMock starts (or restarts) the mock server on the given port.
func (s *DesignService) StartMock(port int) error {
	s.srvMu.Lock()
	if s.srv != nil {
		// Stop the old one first.
		_ = s.srv.Shutdown(context.Background())
		s.srv = nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleMock)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	s.srv = srv
	s.port = port
	s.srvErr = ""
	s.srvMu.Unlock()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.srvMu.Lock()
			s.srvErr = err.Error()
			s.srv = nil
			s.srvMu.Unlock()
		}
	}()
	return nil
}

// StopMock tears down the mock server.
func (s *DesignService) StopMock() {
	s.srvMu.Lock()
	defer s.srvMu.Unlock()
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = s.srv.Shutdown(ctx)
		cancel()
		s.srv = nil
	}
}

// handleMock matches the request against the spec's paths object and returns
// the first example payload it finds.
func (s *DesignService) handleMock(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	spec := s.spec
	s.mu.Unlock()
	if spec == nil {
		http.Error(w, "no spec loaded", http.StatusServiceUnavailable)
		return
	}
	paths, _ := spec["paths"].(map[string]any)
	if paths == nil {
		http.Error(w, "spec has no paths", http.StatusNotFound)
		return
	}

	method := strings.ToLower(r.Method)
	for pathTpl, raw := range paths {
		if !pathMatches(pathTpl, r.URL.Path) {
			continue
		}
		op, _ := raw.(map[string]any)
		if op == nil {
			continue
		}
		method, _ := op[method].(map[string]any)
		if method == nil {
			continue
		}
		if body, ct, status := pickExample(method); body != "" {
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}

// pathMatches resolves OpenAPI path templates ({id}) to concrete URL paths.
func pathMatches(tpl, actual string) bool {
	tParts := strings.Split(strings.Trim(tpl, "/"), "/")
	aParts := strings.Split(strings.Trim(actual, "/"), "/")
	if len(tParts) != len(aParts) {
		return false
	}
	for i := range tParts {
		if strings.HasPrefix(tParts[i], "{") && strings.HasSuffix(tParts[i], "}") {
			continue
		}
		if tParts[i] != aParts[i] {
			return false
		}
	}
	return true
}

// pickExample walks an operation's responses for the first usable example.
func pickExample(op map[string]any) (body, contentType string, status int) {
	responses, _ := op["responses"].(map[string]any)
	if responses == nil {
		return "", "application/json", 200
	}
	// Prefer 2xx codes, then any.
	codes := make([]string, 0, len(responses))
	for k := range responses {
		codes = append(codes, k)
	}
	for _, code := range codes {
		if strings.HasPrefix(code, "2") || code == "default" {
			if b, ct := extractExample(responses[code]); b != "" {
				st, _ := strconv.Atoi(code)
				if st == 0 {
					st = 200
				}
				return b, ct, st
			}
		}
	}
	return "{}", "application/json", 200
}

func extractExample(raw any) (body, contentType string) {
	resp, _ := raw.(map[string]any)
	if resp == nil {
		return "", "application/json"
	}
	content, _ := resp["content"].(map[string]any)
	if content == nil {
		return "", "application/json"
	}
	for ct, mt := range content {
		m, _ := mt.(map[string]any)
		if m == nil {
			continue
		}
		if ex, ok := m["example"]; ok {
			b, _ := json.Marshal(ex)
			return string(b), ct
		}
		if exs, ok := m["examples"].(map[string]any); ok {
			for _, ex := range exs {
				if em, ok := ex.(map[string]any); ok {
					if val, ok := em["value"]; ok {
						b, _ := json.Marshal(val)
						return string(b), ct
					}
				}
			}
		}
	}
	return "", "application/json"
}
