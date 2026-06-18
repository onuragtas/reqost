package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"reqost/internal/collection"
	"reqost/internal/envstore"
	"reqost/internal/httpclient"
	"reqost/internal/script"
	"reqost/internal/update"
)

// CLI subcommands. These run without booting the Wails GUI so the same binary
// can be used in CI ("reqost run collection.json -e env.json") or as a
// local mock server ("reqost mock collection.json --port 8090").

func cliVersion() int {
	fmt.Println("reqost", update.Version)
	return 0
}

func cliHelp() {
	fmt.Print(`reqost — high-performance API client

USAGE:
  reqost                   launch the desktop app
  reqost run <coll.json>   run a Postman collection headlessly
  reqost mock <coll.json>  serve saved examples as a local HTTP mock
  reqost version           print version
  reqost help              this text

`)
}

// ── run ────────────────────────────────────────────────────────────────────

type runReport struct {
	Total  int            `json:"total"`
	Passed int            `json:"passed"`
	Failed int            `json:"failed"`
	Items  []runItemRep   `json:"items"`
	Suite  string         `json:"suite"`
	StartedAt time.Time   `json:"startedAt"`
	EndedAt   time.Time   `json:"endedAt"`
}

type runItemRep struct {
	Name      string             `json:"name"`
	Method    string             `json:"method"`
	URL       string             `json:"url"`
	Status    int                `json:"status"`
	Ok        bool               `json:"ok"`
	DurationMs float64           `json:"durationMs"`
	Error     string             `json:"error,omitempty"`
	Tests     []script.TestResult `json:"tests,omitempty"`
}

func cliRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	envPath := fs.String("e", "", "Postman environment JSON file to apply")
	envFlag := fs.String("env", "", "Alias for -e")
	format  := fs.String("format", "junit", "report format: junit | json | text")
	outPath := fs.String("out", "", "write report to this path (default stdout)")
	verbose := fs.Bool("v", false, "verbose: print each request as it runs")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: reqost run <collection.json> [-e env.json] [--format junit|json|text]")
		return 2
	}
	collectionPath := fs.Arg(0)

	items, _, err := collection.ParseFile(collectionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse collection: %v\n", err)
		return 1
	}

	vars := map[string]string{}
	if p := firstNonEmpty(*envPath, *envFlag); p != "" {
		vs, err := loadEnvFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load env: %v\n", err)
			return 1
		}
		for k, v := range vs {
			vars[k] = v
		}
	}

	client := httpclient.New()
	cache := httpclient.NewCache()
	rep := runReport{Suite: collectionPath, StartedAt: time.Now()}

	for _, it := range items {
		if it.Type != "request" {
			continue
		}
		headers, formFields, auth := parseDetail(it)
		req := httpclient.Request{
			Method:     it.Method,
			URL:        it.URL,
			Headers:    headers,
			Body:       it.Body,
			BodyType:   it.BodyType,
			FormFields: formFields,
			Auth:       auth,
			Variables:  vars,
		}
		httpclient.ResolveResponseRefs(&req, cache.Snapshot())

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// Pre-request script may mutate vars + request.
		if it.PreScript != "" {
			r := script.RunPre(it.PreScript, vars, toScriptRequest(req), nil, script.Info{RequestName: it.Name})
			vars = r.Vars
			req.Variables = vars
			if r.Request != nil {
				applyScriptRequest(&req, r.Request)
			}
		}

		resp, err := client.Execute(ctx, req)
		cancel()
		dur := time.Since(start).Seconds() * 1000

		item := runItemRep{
			Name: it.Name, Method: req.Method, URL: req.URL, DurationMs: dur,
		}
		if err != nil {
			item.Error = err.Error()
			rep.Failed++
		} else {
			item.Status = resp.Status
			item.Ok = resp.Status >= 200 && resp.Status < 400
			cache.Put(it.Name, httpclient.LastResponse{Status: resp.Status, Body: resp.Body, Headers: resp.Headers})

			// Test script.
			if it.PostScript != "" {
				r := script.RunTests(it.PostScript, vars, toScriptResponse(resp), nil, script.Info{RequestName: it.Name})
				item.Tests = r.Tests
				vars = r.Vars
				for _, tr := range r.Tests {
					if !tr.Passed {
						rep.Failed++
					} else {
						rep.Passed++
					}
				}
			} else if item.Ok {
				rep.Passed++
			} else {
				rep.Failed++
			}
		}
		rep.Items = append(rep.Items, item)
		rep.Total++

		if *verbose {
			marker := "✓"
			if !item.Ok || item.Error != "" {
				marker = "✗"
			}
			fmt.Fprintf(os.Stderr, "%s %s %s — %d (%.0f ms)\n", marker, item.Method, item.Name, item.Status, item.DurationMs)
		}
	}
	rep.EndedAt = time.Now()

	out := os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create out: %v\n", err)
			return 1
		}
		defer f.Close()
		out = f
	}

	switch *format {
	case "json":
		_ = json.NewEncoder(out).Encode(rep)
	case "text":
		fmt.Fprintf(out, "reqost run %s — %d requests, %d passed, %d failed (%.1fs)\n",
			rep.Suite, rep.Total, rep.Passed, rep.Failed, rep.EndedAt.Sub(rep.StartedAt).Seconds())
	default: // junit
		writeJUnit(out, rep)
	}

	if rep.Failed > 0 {
		return 1
	}
	return 0
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func loadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Values []envstore.Var `json:"values"`
		Vars   []envstore.Var `json:"vars"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	src := doc.Values
	if len(src) == 0 {
		src = doc.Vars
	}
	out := map[string]string{}
	for _, v := range src {
		if v.Enabled || v.Key == "" {
			out[v.Key] = v.Value
		}
	}
	return out, nil
}

func parseDetail(it collection.FlatItem) ([]httpclient.Header, []httpclient.FormField, *httpclient.Auth) {
	var hs []httpclient.Header
	if it.HeadersJSON != "" {
		_ = json.Unmarshal([]byte(it.HeadersJSON), &hs)
	}
	var ff []httpclient.FormField
	if it.FormFields != "" {
		_ = json.Unmarshal([]byte(it.FormFields), &ff)
	}
	var auth *httpclient.Auth
	if it.AuthJSON != "" {
		var a httpclient.Auth
		if err := json.Unmarshal([]byte(it.AuthJSON), &a); err == nil && a.Type != "" && a.Type != "none" {
			auth = &a
		}
	}
	return hs, ff, auth
}

func writeJUnit(w *os.File, rep runReport) {
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(w, `<testsuite name="%s" tests="%d" failures="%d" time="%.3f">`+"\n",
		escXML(rep.Suite), rep.Total, rep.Failed, rep.EndedAt.Sub(rep.StartedAt).Seconds())
	for _, it := range rep.Items {
		name := fmt.Sprintf("%s %s", it.Method, it.Name)
		fmt.Fprintf(w, `  <testcase classname="%s" name="%s" time="%.3f">`+"\n",
			escXML(rep.Suite), escXML(name), it.DurationMs/1000)
		if it.Error != "" {
			fmt.Fprintf(w, `    <failure message="%s"/>`+"\n", escXML(it.Error))
		}
		for _, tr := range it.Tests {
			if !tr.Passed {
				fmt.Fprintf(w, `    <failure message="%s">%s</failure>`+"\n",
					escXML(tr.Name), escXML(tr.Error))
			}
		}
		fmt.Fprintln(w, `  </testcase>`)
	}
	fmt.Fprintln(w, `</testsuite>`)
}

func escXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// ── mock ───────────────────────────────────────────────────────────────────

func cliMock(args []string) int {
	fs := flag.NewFlagSet("mock", flag.ExitOnError)
	port := fs.Int("port", 8090, "port to listen on")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: reqost mock <collection.json> [--port 8090]")
		return 2
	}
	collectionPath := fs.Arg(0)

	// Collection export shape isn't quite our index; reuse the parser then
	// resolve saved examples from each request's `response` array.
	items, _, err := collection.ParseFile(collectionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse collection: %v\n", err)
		return 1
	}

	type mockEntry struct {
		Name    string
		Method  string
		Path    string
		Status  int
		Headers []httpclient.Header
		Body    string
	}
	// `collection.ParseFile` returns flat items but doesn't carry Postman
	// example arrays — for the minimum viable mock we serve the request's
	// own (method, URL path) → 200 OK with a "mock" body. Hooking actual
	// saved Postman examples is a follow-up once the parser surfaces them.
	entries := make([]mockEntry, 0, len(items))
	for _, it := range items {
		if it.Type != "request" || it.URL == "" {
			continue
		}
		path := urlPath(it.URL)
		if path == "" {
			continue
		}
		entries = append(entries, mockEntry{
			Name:   it.Name,
			Method: strings.ToUpper(it.Method),
			Path:   path,
			Status: 200,
			Body:   `{"mocked": true, "endpoint": "` + path + `"}`,
			Headers: []httpclient.Header{{Key: "Content-Type", Value: "application/json", Enabled: true}},
		})
	}

	mux := http.NewServeMux()
	for i := range entries {
		e := entries[i]
		mux.HandleFunc(e.Path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != e.Method && e.Method != "" {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			for _, h := range e.Headers {
				w.Header().Set(h.Key, h.Value)
			}
			w.WriteHeader(e.Status)
			_, _ = w.Write([]byte(e.Body))
		})
	}

	addr := fmt.Sprintf(":%d", *port)
	fmt.Fprintf(os.Stderr, "reqost mock — %d endpoints on http://localhost%s\n", len(entries), addr)
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "  %-6s %s  (%s)\n", e.Method, e.Path, e.Name)
	}
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		return 1
	}
	return 0
}

func urlPath(rawurl string) string {
	if i := strings.Index(rawurl, "://"); i >= 0 {
		rest := rawurl[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			p := rest[j:]
			if q := strings.Index(p, "?"); q >= 0 {
				return p[:q]
			}
			return p
		}
		return "/"
	}
	if strings.HasPrefix(rawurl, "/") {
		return rawurl
	}
	return ""
}
