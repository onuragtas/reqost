package httpclient

// Header is an ordered, toggleable key/value pair (Postman-style). The same
// shape is used for both request and response headers so the frontend editor
// and viewer can share a component.
type Header struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// Auth describes how to authenticate a request. The resolved credential is
// applied as a header (query placement comes later). Fields support {{vars}}.
type Auth struct {
	Type     string `json:"type"`     // "none" | "bearer" | "basic" | "apikey"
	Token    string `json:"token"`    // bearer
	Username string `json:"username"` // basic
	Password string `json:"password"` // basic
	Key      string `json:"key"`      // apikey: header name
	Value    string `json:"value"`    // apikey: header value
}

// FormField is one row of a urlencoded or multipart form body. For Type=="file"
// Value is a local filesystem path that gets streamed into a multipart part.
type FormField struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type"` // "text" | "file"
	Enabled bool   `json:"enabled"`
}

// Request is the protocol-agnostic input to an executor. Only "http" is
// implemented today; the Protocol field reserves room for graphql/ws/grpc,
// which slot in behind the same Execute entry point later.
//
// Per-request execution options use inverted boolean names so the zero value
// keeps the safe defaults (follow redirects, verify TLS, no timeout).
type Request struct {
	Protocol           string            `json:"protocol"` // "" or "http" for now
	Method             string            `json:"method"`
	URL                string            `json:"url"`
	Headers            []Header          `json:"headers"`
	Body               string            `json:"body"`     // raw/json bodies
	BodyType           string            `json:"bodyType"` // "none" | "raw" | "json" | "urlencoded" | "formdata"
	FormFields         []FormField       `json:"formFields"`
	Auth               *Auth             `json:"auth"`
	Variables          map[string]string `json:"variables"`          // resolved environment vars for {{interpolation}}
	TimeoutMs          int               `json:"timeoutMs"`          // 0 = no timeout
	DisableRedirect    bool              `json:"disableRedirect"`    // false = follow redirects
	MaxRedirects       int               `json:"maxRedirects"`       // 0 = stdlib default (10)
	InsecureSkipVerify bool               `json:"insecureSkipVerify"` // false = verify TLS
}

// Timing is the per-phase breakdown captured via net/http/httptrace, in
// milliseconds. Zero means the phase did not occur (e.g. TLS on plain HTTP, or
// a reused keep-alive connection that skipped DNS/connect).
type Timing struct {
	DNSMs     float64 `json:"dnsMs"`
	ConnectMs float64 `json:"connectMs"`
	TLSMs     float64 `json:"tlsMs"`
	TTFBMs    float64 `json:"ttfbMs"`
	TotalMs   float64 `json:"totalMs"`
}

// Response is what the UI renders: status, headers, body and timing/size.
type Response struct {
	Status     int      `json:"status"`
	StatusText string   `json:"statusText"`
	Headers    []Header `json:"headers"`
	Body       string   `json:"body"`
	SizeBytes  int64    `json:"sizeBytes"`
	Timing     Timing   `json:"timing"`
}
