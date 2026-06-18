package main

import (
	"context"
	"sync"

	"reqost/internal/httpclient"
	"reqost/internal/plugins"
	"reqost/internal/script"
)

// ExecService is the Wails service that runs requests. Kept separate from
// CollectionService so request execution and the collection index stay
// decoupled as more protocols (graphql/ws/grpc) are added.
type ExecService struct {
	client *httpclient.Client

	mu       sync.Mutex
	inFlight map[string]context.CancelFunc // reqId -> cancel

	// respCache backs Insomnia-style {{Name.response.body.path}} references.
	// Keyed by the request's display name (passed from the frontend).
	respCache *httpclient.Cache

	// pluginSvc is optional — when set, enabled plugin hooks run around
	// every request. Wired in main.go after construction so the dependency
	// graph stays flat (ExecService doesn't import plugins directly).
	pluginSvc *PluginService
}

func NewExecService() *ExecService {
	return &ExecService{
		client:    httpclient.New(),
		inFlight:  make(map[string]context.CancelFunc),
		respCache: httpclient.NewCache(),
	}
}

func (s *ExecService) setPluginSvc(p *PluginService) { s.pluginSvc = p }

// SendResult bundles the response with script side-effects so the UI can show
// test results and persist any variable changes a script made.
type SendResult struct {
	Response    *httpclient.Response `json:"response"`
	Tests       []script.TestResult  `json:"tests"`
	Logs        []string             `json:"logs"`
	Vars        map[string]string    `json:"vars"`        // variable map after pre/post scripts
	ScriptError string               `json:"scriptError"` // non-fatal: a script threw
}

// SendRequest runs the optional pre-request script, executes the request, then
// runs the optional test script. reqId lets the UI cancel via Cancel. reqName
// is the request's display name — used as the key for
// `{{Name.response.body.path}}` chaining references.
func (s *ExecService) SendRequest(reqId, reqName string, req httpclient.Request, preScript, postScript string) (*SendResult, error) {
	ctx := context.Background()
	if reqId != "" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		s.mu.Lock()
		s.inFlight[reqId] = cancel
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(s.inFlight, reqId)
			s.mu.Unlock()
			cancel()
		}()
	}

	vars := req.Variables
	if vars == nil {
		vars = map[string]string{}
	}
	out := &SendResult{Vars: vars}

	// Resolve {{Name.response.body.path}} references using the cache of past
	// responses *before* the pre-request script runs, so scripts see the
	// already-interpolated values too.
	req.Variables = vars
	httpclient.ResolveResponseRefs(&req, s.respCache.Snapshot())
	vars = req.Variables

	sender := s.makeSender(ctx)
	info := script.Info{RequestID: reqId, RequestName: reqName}

	// Pre-request script: may mutate variables and the request.
	if preScript != "" {
		pre := script.RunPre(preScript, vars, toScriptRequest(req), sender, info)
		vars = pre.Vars
		out.Vars = vars
		out.Logs = append(out.Logs, pre.Logs...)
		if pre.Error != "" {
			out.ScriptError = pre.Error
		}
		if pre.Request != nil {
			applyScriptRequest(&req, pre.Request)
		}
	}
	req.Variables = vars

	// Plugin pre-send hooks. Plugins see a plain map (JSON-like) and may
	// mutate URL/headers/body. We materialize their changes back into req.
	var loadedPlugins []plugins.Loaded
	if s.pluginSvc != nil && s.pluginSvc.manager() != nil {
		loadedPlugins, _ = s.pluginSvc.manager().LoadEnabled()
		if len(loadedPlugins) > 0 {
			mutated := plugins.RunPreSend(loadedPlugins, requestToMap(req))
			applyMapToRequest(&req, mutated)
		}
	}

	resp, err := s.client.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	out.Response = resp

	// Plugin post-receive hooks (observers only — return value ignored).
	if len(loadedPlugins) > 0 {
		plugins.RunPostReceive(loadedPlugins, requestToMap(req), responseToMap(resp))
	}

	// Persist this response under its display name so future requests can
	// chain off it. Empty name (e.g. adhoc tabs from History) skipped.
	if reqName != "" {
		s.respCache.Put(reqName, httpclient.LastResponse{
			Status:  resp.Status,
			Body:    resp.Body,
			Headers: resp.Headers,
		})
	}

	// Test script: asserts against the response.
	if postScript != "" {
		test := script.RunTests(postScript, vars, toScriptResponse(resp), sender, info)
		out.Vars = test.Vars
		out.Tests = test.Tests
		out.Logs = append(out.Logs, test.Logs...)
		if test.Error != "" && out.ScriptError == "" {
			out.ScriptError = test.Error
		}
	}
	return out, nil
}

// GetCookies returns the cookies the session jar would send to url.
func (s *ExecService) GetCookies(url string) []httpclient.Cookie {
	return s.client.Cookies(url)
}

// ClearCookies empties the session cookie jar.
func (s *ExecService) ClearCookies() {
	s.client.ClearCookies()
}

// Cancel aborts an in-flight request started with the given reqId.
func (s *ExecService) Cancel(reqId string) {
	s.mu.Lock()
	cancel := s.inFlight[reqId]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// makeSender backs pm.sendRequest: scripts can fire an HTTP request through the
// same client (shared cookie jar), bounded by the request's context.
func (s *ExecService) makeSender(ctx context.Context) script.Sender {
	return func(in script.SendInput) script.SendOutput {
		hs := make([]httpclient.Header, 0, len(in.Headers))
		for _, h := range in.Headers {
			hs = append(hs, httpclient.Header{Key: h.Key, Value: h.Value, Enabled: true})
		}
		bodyType := "none"
		if in.Body != "" {
			bodyType = "raw"
		}
		resp, err := s.client.Execute(ctx, httpclient.Request{
			Method: in.Method, URL: in.URL, Headers: hs, Body: in.Body, BodyType: bodyType,
		})
		if err != nil {
			return script.SendOutput{Error: err.Error()}
		}
		out := script.SendOutput{Code: resp.Status, Status: resp.StatusText, Body: resp.Body}
		for _, h := range resp.Headers {
			out.Headers = append(out.Headers, script.KV{Key: h.Key, Value: h.Value})
		}
		return out
	}
}

func toScriptRequest(req httpclient.Request) script.ScriptRequest {
	return script.ScriptRequest{
		Method: req.Method, URL: req.URL, Body: req.Body,
		Headers: headersToKV(req.Headers),
	}
}

func toScriptResponse(resp *httpclient.Response) script.ScriptResponse {
	return script.ScriptResponse{
		Code: resp.Status, Status: resp.StatusText,
		ResponseTime: resp.Timing.TotalMs, Body: resp.Body,
		Headers: headersToKV(resp.Headers),
	}
}

func headersToKV(hs []httpclient.Header) []script.KV {
	out := make([]script.KV, 0, len(hs))
	for _, h := range hs {
		out = append(out, script.KV{Key: h.Key, Value: h.Value})
	}
	return out
}

// requestToMap is the shape plugin authors see — a plain JS-friendly object.
func requestToMap(req httpclient.Request) map[string]any {
	hs := make([]map[string]any, 0, len(req.Headers))
	for _, h := range req.Headers {
		hs = append(hs, map[string]any{"key": h.Key, "value": h.Value, "enabled": h.Enabled})
	}
	return map[string]any{
		"method":  req.Method,
		"url":     req.URL,
		"headers": hs,
		"body":    req.Body,
	}
}

func responseToMap(resp *httpclient.Response) map[string]any {
	if resp == nil {
		return map[string]any{}
	}
	hs := make([]map[string]any, 0, len(resp.Headers))
	for _, h := range resp.Headers {
		hs = append(hs, map[string]any{"key": h.Key, "value": h.Value})
	}
	return map[string]any{
		"status":     resp.Status,
		"statusText": resp.StatusText,
		"headers":    hs,
		"body":       resp.Body,
	}
}

// applyMapToRequest pulls method/url/headers/body back out of a plugin's
// mutated map (others are ignored — plugins can't inject new top-level fields).
func applyMapToRequest(req *httpclient.Request, m map[string]any) {
	if v, ok := m["method"].(string); ok && v != "" {
		req.Method = v
	}
	if v, ok := m["url"].(string); ok && v != "" {
		req.URL = v
	}
	if v, ok := m["body"].(string); ok {
		req.Body = v
	}
	if raw, ok := m["headers"].([]any); ok {
		hs := make([]httpclient.Header, 0, len(raw))
		for _, e := range raw {
			if h, ok := e.(map[string]any); ok {
				k, _ := h["key"].(string)
				val, _ := h["value"].(string)
				enabled, _ := h["enabled"].(bool)
				if k != "" {
					if _, hasEnabled := h["enabled"]; !hasEnabled {
						enabled = true
					}
					hs = append(hs, httpclient.Header{Key: k, Value: val, Enabled: enabled})
				}
			}
		}
		req.Headers = hs
	}
}

// applyScriptRequest writes a pre-request script's mutations back onto req.
func applyScriptRequest(req *httpclient.Request, sr *script.ScriptRequest) {
	req.Method = sr.Method
	req.URL = sr.URL
	req.Body = sr.Body
	hs := make([]httpclient.Header, 0, len(sr.Headers))
	for _, h := range sr.Headers {
		hs = append(hs, httpclient.Header{Key: h.Key, Value: h.Value, Enabled: true})
	}
	req.Headers = hs
}
