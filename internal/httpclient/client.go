package httpclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"
)

// maxBodySize caps how much of a response body we buffer, so a multi-gigabyte
// download can't OOM the app. Bodies are truncated past this point.
const maxBodySize = 50 << 20 // 50 MiB

// Client executes requests. It holds a shared cookie jar and two pooled
// transports (secure / TLS-insecure). Each Execute builds a thin http.Client
// around the chosen transport so per-call redirect policy and timeout are
// honored without losing connection pooling.
type Client struct {
	jar             http.CookieJar
	secureTransp    http.RoundTripper
	insecureTransp  http.RoundTripper

	// proxyCache memoizes transports keyed by "secure?proxyURL" so requests
	// routed through the same proxy reuse their connection pool. Per-request
	// proxy override is rare in practice but should still pool when it
	// happens (e.g. CI fanning out via one corporate proxy).
	proxyCache sync.Map // map[string]http.RoundTripper
}

func New() *Client {
	// A session cookie jar so Set-Cookie responses are sent back automatically
	// on later requests to the same host (like a browser / Postman).
	jar, _ := cookiejar.New(nil)
	return &Client{
		jar:            jar,
		secureTransp:   http.DefaultTransport,
		insecureTransp: insecureTransport(),
	}
}

// transportFor returns the transport for one Execute call, picking the secure
// or TLS-insecure pool and overlaying a custom proxy and/or mTLS client
// certificates when the request asks for them.
func (c *Client) transportFor(req Request) http.RoundTripper {
	clientCert := matchClientCert(req.ClientCerts, req.URL)

	if req.ProxyURL == "" && clientCert == nil {
		if req.InsecureSkipVerify {
			return c.insecureTransp
		}
		return c.secureTransp
	}

	// With mTLS we always build a fresh transport so the cert lives only on
	// this request's pool — switching identities mid-session shouldn't leak
	// the previous one into a pooled connection.
	if clientCert != nil {
		t := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{*clientCert},
				InsecureSkipVerify: req.InsecureSkipVerify, //nolint:gosec
			},
		}
		if req.ProxyURL != "" {
			if u, err := url.Parse(req.ProxyURL); err == nil {
				t.Proxy = http.ProxyURL(u)
			}
		}
		return t
	}

	key := req.ProxyURL
	if req.InsecureSkipVerify {
		key = "insecure:" + key
	}
	if cached, ok := c.proxyCache.Load(key); ok {
		return cached.(http.RoundTripper)
	}

	u, err := url.Parse(req.ProxyURL)
	if err != nil {
		// Bad proxy URL → fall back to no proxy. The host:port itself will
		// surface the error in the response.
		if req.InsecureSkipVerify {
			return c.insecureTransp
		}
		return c.secureTransp
	}

	t := &http.Transport{
		Proxy: http.ProxyURL(u),
	}
	if req.InsecureSkipVerify {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec — opt-in
	}
	actual, _ := c.proxyCache.LoadOrStore(key, http.RoundTripper(t))
	return actual.(http.RoundTripper)
}

func insecureTransport() http.RoundTripper {
	return &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec — opt-in via per-request flag
	}
}

// matchClientCert returns the certificate to present for the request URL, or
// nil if no pattern matches. Loading happens here (per-call) so the user can
// edit the cert on disk and pick it up without restarting reqost.
func matchClientCert(certs []ClientCert, rawurl string) *tls.Certificate {
	if len(certs) == 0 {
		return nil
	}
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, c := range certs {
		if !hostMatches(host, c.HostPattern) {
			continue
		}
		cert, err := tls.LoadX509KeyPair(c.CertPath, c.KeyPath)
		if err != nil {
			continue
		}
		return &cert
	}
	return nil
}

// hostMatches supports plain hostnames ("api.example.com"), wildcard prefixes
// ("*.example.com" → ".example.com" suffix), and bare suffixes ("example.com").
func hostMatches(host, pattern string) bool {
	if pattern == "" {
		return false
	}
	p := strings.ToLower(pattern)
	if strings.HasPrefix(p, "*.") {
		p = p[1:] // -> ".example.com"
	}
	if strings.HasPrefix(p, ".") {
		return strings.HasSuffix(host, p) || host == strings.TrimPrefix(p, ".")
	}
	return host == p
}

// ErrTooManyRedirects is returned when MaxRedirects is reached.
var ErrTooManyRedirects = errors.New("stopped after max redirects")

// Execute sends an HTTP request and returns the response with a timing
// breakdown. The context controls cancellation; req.TimeoutMs adds a deadline.
func (c *Client) Execute(ctx context.Context, req Request) (*Response, error) {
	if req.Protocol != "" && req.Protocol != "http" {
		return nil, fmt.Errorf("unsupported protocol %q", req.Protocol)
	}

	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	vars := req.Variables
	url := interpolate(req.URL, vars)

	body, contentType, err := buildBody(req, method, vars)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for _, h := range req.Headers {
		if !h.Enabled || h.Key == "" {
			continue
		}
		httpReq.Header.Add(interpolate(h.Key, vars), interpolate(h.Value, vars))
	}
	// Set the body's Content-Type unless the user already specified one.
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	applyAuth(httpReq, req.Auth, vars)

	var t timings
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(httpReq.Context(), t.trace()))

	transport := c.transportFor(req)
	maxRedirects := req.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 10
	}
	hc := &http.Client{
		Jar:       c.jar,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if req.DisableRedirect {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	end := time.Now()

	return &Response{
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		Headers:    headerSlice(resp.Header),
		Body:       string(bodyBytes),
		SizeBytes:  int64(len(bodyBytes)),
		Timing:     t.toTiming(start, end),
	}, nil
}

// applyAuth sets the auth header derived from a. Variables are interpolated so
// tokens/passwords can reference {{vars}}. A header the user already set wins
// is not special-cased — auth overwrites a same-named header.
func applyAuth(req *http.Request, a *Auth, vars map[string]string) {
	if a == nil {
		return
	}
	switch a.Type {
	case "bearer":
		if t := interpolate(a.Token, vars); t != "" {
			req.Header.Set("Authorization", "Bearer "+t)
		}
	case "basic":
		u := interpolate(a.Username, vars)
		p := interpolate(a.Password, vars)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u+":"+p)))
	case "apikey":
		if k := interpolate(a.Key, vars); k != "" {
			req.Header.Set(k, interpolate(a.Value, vars))
		}
	}
}

func headerSlice(h http.Header) []Header {
	out := make([]Header, 0, len(h))
	for k, vs := range h {
		for _, v := range vs {
			out = append(out, Header{Key: k, Value: v, Enabled: true})
		}
	}
	return out
}

// timings records httptrace callback timestamps. Callbacks may fire on a
// different goroutine than Do's caller, but Do blocks until the response is
// returned, so all writes happen-before we read them in toTiming.
type timings struct {
	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	tlsStart, tlsDone         time.Time
	firstByte                 time.Time
}

func (t *timings) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { t.dnsStart = time.Now() },
		DNSDone:  func(httptrace.DNSDoneInfo) { t.dnsDone = time.Now() },
		ConnectStart: func(_, _ string) {
			if t.connectStart.IsZero() {
				t.connectStart = time.Now()
			}
		},
		ConnectDone:          func(_, _ string, _ error) { t.connectDone = time.Now() },
		TLSHandshakeStart:    func() { t.tlsStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { t.tlsDone = time.Now() },
		GotFirstResponseByte: func() { t.firstByte = time.Now() },
	}
}

func (t *timings) toTiming(start, end time.Time) Timing {
	return Timing{
		DNSMs:     ms(t.dnsStart, t.dnsDone),
		ConnectMs: ms(t.connectStart, t.connectDone),
		TLSMs:     ms(t.tlsStart, t.tlsDone),
		TTFBMs:    ms(start, t.firstByte),
		TotalMs:   ms(start, end),
	}
}

// ms returns b-a in fractional milliseconds, or 0 if either bound is unset or
// out of order (e.g. a keep-alive connection that skipped DNS/connect).
func ms(a, b time.Time) float64 {
	if a.IsZero() || b.IsZero() || b.Before(a) {
		return 0
	}
	return float64(b.Sub(a).Microseconds()) / 1000.0
}
