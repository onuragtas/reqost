// Package script runs Postman-style pre-request and test scripts in a goja
// (pure-Go JS) sandbox exposing a pragmatic subset of the pm.* API.
package script

import (
	"encoding/base64"
	"time"

	"github.com/dop251/goja"
)

const runTimeout = 2 * time.Second

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ScriptRequest is the mutable request a pre-request script can edit.
type ScriptRequest struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Body    string `json:"body"`
	Headers []KV   `json:"headers"`
}

// ScriptResponse is the read-only response a test script inspects.
type ScriptResponse struct {
	Code         int     `json:"code"`
	Status       string  `json:"status"`
	ResponseTime float64 `json:"responseTime"`
	Body         string  `json:"body"`
	Headers      []KV    `json:"headers"`
}

type TestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error"`
}

// SendInput/SendOutput/Sender let scripts make HTTP calls via pm.sendRequest
// without the script package importing the http client (no import cycle).
type SendInput struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Body    string `json:"body"`
	Headers []KV   `json:"headers"`
}
type SendOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Body    string `json:"body"`
	Headers []KV   `json:"headers"`
	Error   string `json:"error"`
}
type Sender func(SendInput) SendOutput

// Result is the outcome of a script run.
type Result struct {
	Vars    map[string]string `json:"vars"`    // possibly-mutated variable map
	Tests   []TestResult      `json:"tests"`   // from pm.test / tests{}
	Logs    []string          `json:"logs"`    // console.log output
	Error   string            `json:"error"`   // uncaught top-level error, if any
	Request *ScriptRequest    `json:"request"` // mutated request (pre-request only)
}

// Info carries request metadata exposed to scripts as pm.info.
type Info struct {
	RequestName string
	RequestID   string
}

// RunPre executes a pre-request script. It may mutate vars and the request.
// send may be nil to disable pm.sendRequest.
func RunPre(src string, vars map[string]string, req ScriptRequest, send Sender, info Info) Result {
	if src == "" {
		return Result{Vars: vars}
	}
	return run(src, vars, &req, nil, send, info, "prerequest")
}

// RunTests executes a test script against a response, producing test results.
func RunTests(src string, vars map[string]string, resp ScriptResponse, send Sender, info Info) Result {
	if src == "" {
		return Result{Vars: vars}
	}
	return run(src, vars, nil, &resp, send, info, "test")
}

func run(src string, vars map[string]string, req *ScriptRequest, resp *ScriptResponse, send Sender, info Info, eventName string) Result {
	vm := goja.New()

	mutated := map[string]string{}
	for k, v := range vars {
		mutated[k] = v
	}
	res := Result{Vars: mutated}

	host := vm.NewObject()
	_ = host.Set("getEnv", func(k string) string { return mutated[k] })
	_ = host.Set("setEnv", func(k, v string) { mutated[k] = v })
	_ = host.Set("unsetEnv", func(k string) { delete(mutated, k) })
	_ = host.Set("log", func(s string) { res.Logs = append(res.Logs, s) })
	_ = host.Set("addTest", func(name string, passed bool, errMsg string) {
		res.Tests = append(res.Tests, TestResult{Name: name, Passed: passed, Error: errMsg})
	})
	_ = host.Set("btoa", func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) })
	_ = host.Set("atob", func(s string) string {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return ""
		}
		return string(b)
	})
	_ = host.Set("envObject", func() map[string]string {
		out := make(map[string]string, len(mutated))
		for k, v := range mutated {
			out[k] = v
		}
		return out
	})
	_ = host.Set("sendRequest", func(in map[string]any) map[string]any {
		if send == nil {
			return map[string]any{"error": "pm.sendRequest is not available here"}
		}
		out := send(SendInput{
			Method:  str(in["method"]),
			URL:     str(in["url"]),
			Body:    str(in["body"]),
			Headers: anyToKV(in["headers"]),
		})
		hdrs := make([]map[string]string, 0, len(out.Headers))
		for _, h := range out.Headers {
			hdrs = append(hdrs, map[string]string{"key": h.Key, "value": h.Value})
		}
		return map[string]any{
			"code": out.Code, "status": out.Status, "body": out.Body,
			"headers": hdrs, "error": out.Error,
		}
	})
	_ = vm.Set("__host", host)

	infoObj := vm.NewObject()
	_ = infoObj.Set("eventName", eventName)
	_ = infoObj.Set("requestName", info.RequestName)
	_ = infoObj.Set("requestId", info.RequestID)
	_ = infoObj.Set("iteration", 0)
	_ = infoObj.Set("iterationCount", 1)
	_ = vm.Set("__info", infoObj)

	var reqObj *goja.Object
	if req != nil {
		reqObj = vm.NewObject()
		_ = reqObj.Set("method", req.Method)
		_ = reqObj.Set("url", req.URL)
		_ = reqObj.Set("body", req.Body)
		_ = reqObj.Set("headers", kvToJS(vm, req.Headers))
		_ = vm.Set("__request", reqObj)
	} else {
		_ = vm.Set("__request", goja.Undefined())
	}

	if resp != nil {
		respObj := vm.NewObject()
		_ = respObj.Set("code", resp.Code)
		_ = respObj.Set("status", resp.Status)
		_ = respObj.Set("responseTime", resp.ResponseTime)
		_ = respObj.Set("body", resp.Body)
		_ = respObj.Set("headers", kvToJS(vm, resp.Headers))
		_ = vm.Set("__response", respObj)
	} else {
		_ = vm.Set("__response", goja.Undefined())
	}

	// Guard against runaway scripts.
	t := time.AfterFunc(runTimeout, func() { vm.Interrupt("script timeout") })
	defer t.Stop()

	if _, err := vm.RunString(prelude); err != nil {
		res.Error = "sandbox init: " + err.Error()
		return res
	}
	if _, err := vm.RunString(src); err != nil {
		res.Error = err.Error()
		// still flush legacy tests below
	}
	if _, err := vm.RunString(epilogue); err != nil && res.Error == "" {
		res.Error = err.Error()
	}

	if reqObj != nil {
		res.Request = &ScriptRequest{
			Method:  reqObj.Get("method").String(),
			URL:     reqObj.Get("url").String(),
			Body:    reqObj.Get("body").String(),
			Headers: jsToKV(vm, reqObj.Get("headers")),
		}
	}
	return res
}

func kvToJS(vm *goja.Runtime, kvs []KV) goja.Value {
	arr := make([]map[string]string, 0, len(kvs))
	for _, kv := range kvs {
		arr = append(arr, map[string]string{"key": kv.Key, "value": kv.Value})
	}
	return vm.ToValue(arr)
}

func jsToKV(vm *goja.Runtime, v goja.Value) []KV {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	var raw []map[string]any
	if err := vm.ExportTo(v, &raw); err != nil {
		return nil
	}
	out := make([]KV, 0, len(raw))
	for _, m := range raw {
		out = append(out, KV{Key: str(m["key"]), Value: str(m["value"])})
	}
	return out
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// anyToKV converts a JS headers value (array of {key,value} or a plain object)
// into []KV.
func anyToKV(v any) []KV {
	var out []KV
	switch h := v.(type) {
	case []any:
		for _, e := range h {
			if m, ok := e.(map[string]any); ok {
				out = append(out, KV{Key: str(m["key"]), Value: str(m["value"])})
			}
		}
	case map[string]any:
		for k, val := range h {
			out = append(out, KV{Key: k, Value: str(val)})
		}
	}
	return out
}
