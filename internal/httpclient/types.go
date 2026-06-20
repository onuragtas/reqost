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
	Type     string `json:"type"`     // "none" | "bearer" | "basic" | "apikey" | "jwt" | "digest"
	Token    string `json:"token"`    // bearer (and pre-baked JWT once we mint it client-side)
	Username string `json:"username"` // basic / digest
	Password string `json:"password"` // basic / digest
	Key      string `json:"key"`      // apikey: header name
	Value    string `json:"value"`    // apikey: header value

	// JWT-specific. The frontend builds the actual token and assigns it to
	// `Token`; these are kept on the struct so a re-send can refresh the
	// token if the caller wants to (e.g. when `iat`/`exp` drift).
	JWTAlgo   string `json:"jwtAlgo,omitempty"`   // "HS256" | "HS384" | "HS512"
	JWTSecret string `json:"jwtSecret,omitempty"`
	JWTClaims string `json:"jwtClaims,omitempty"` // JSON payload
}

// FormField is one row of a urlencoded or multipart form body. For Type=="file"
// Value is a local filesystem path that gets streamed into a multipart part.
//
// ContentType overrides the default per-part Content-Type — important when
// the API expects a JSON part alongside a binary file upload (e.g. SaaS
// "upload + metadata" endpoints).
type FormField struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Type        string `json:"type"` // "text" | "file"
	Enabled     bool   `json:"enabled"`
	ContentType string `json:"contentType,omitempty"`
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
	InsecureSkipVerify bool              `json:"insecureSkipVerify"` // false = verify TLS
	ProxyURL           string            `json:"proxyURL"`           // empty = system proxy from env
	ClientCerts        []ClientCert      `json:"clientCerts"`        // mTLS: cert+key matched against URL host
	CAFilePath         string            `json:"caFilePath"`         // PEM bundle to trust in addition to the system roots; empty = system-only
}

// ClientCert is a host-pattern → certificate mapping. HostPattern matches via
// case-insensitive suffix (so "*.example.com" handled as ".example.com" suffix
// match — full glob support is overkill for this UI). The first matching cert
// is presented during the TLS handshake.
type ClientCert struct {
	HostPattern string `json:"hostPattern"` // e.g. "api.example.com", "*.corp.local", or ".internal"
	CertPath    string `json:"certPath"`
	KeyPath     string `json:"keyPath"`
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
