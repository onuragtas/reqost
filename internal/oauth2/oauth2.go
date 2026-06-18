// Package oauth2 runs the OAuth 2.0 token acquisition flows reqost needs at
// request time. Three grants are supported in the order they're most common
// for API testing:
//
//   - Client Credentials  (machine-to-machine, no browser)
//   - Password            (legacy ROPC, kept for legacy intranet APIs)
//   - Authorization Code with PKCE (interactive — opens the system browser
//     and listens on a transient localhost callback)
//
// Tokens are returned to the caller; persistence (cache by config hash) lives
// in the service layer so plugins / runner can share the cache too.
package oauth2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GrantType is the OAuth 2 grant being exchanged.
type GrantType string

const (
	GrantClientCredentials GrantType = "client_credentials"
	GrantPassword          GrantType = "password"
	GrantAuthCode          GrantType = "authorization_code"
)

// Config carries the fields needed to obtain (or refresh) a token. Not every
// field applies to every grant — the runner checks required fields per grant.
type Config struct {
	Grant         GrantType `json:"grant"`
	AuthURL       string    `json:"authUrl"`       // auth code grant only
	TokenURL      string    `json:"tokenUrl"`
	ClientID      string    `json:"clientId"`
	ClientSecret  string    `json:"clientSecret"`
	Username      string    `json:"username"`      // password grant
	Password      string    `json:"password"`      // password grant
	Scope         string    `json:"scope"`
	Audience      string    `json:"audience"`      // RFC 8693 / Auth0
	RedirectURI   string    `json:"redirectUri"`   // auth code; empty → http://localhost:<picked>/callback
	UsePKCE       bool      `json:"usePkce"`       // auth code; defaults true if unset
	ClientAuthIn  string    `json:"clientAuthIn"`  // "header" (default) | "body"
}

// Token is the parsed OAuth response.
type Token struct {
	AccessToken  string    `json:"accessToken"`
	TokenType    string    `json:"tokenType"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

// Get acquires a fresh token according to cfg. For authorization_code, this
// will open the user's browser via openBrowser (caller supplies, so the Wails
// app uses runtime.BrowserOpen and tests can stub).
func Get(ctx context.Context, cfg Config, openBrowser func(string) error) (*Token, error) {
	switch cfg.Grant {
	case GrantClientCredentials:
		return doClientCredentials(ctx, cfg)
	case GrantPassword:
		return doPassword(ctx, cfg)
	case GrantAuthCode:
		return doAuthCode(ctx, cfg, openBrowser)
	default:
		return nil, fmt.Errorf("unsupported grant %q", cfg.Grant)
	}
}

// Refresh exchanges a refresh token for a fresh access token.
func Refresh(ctx context.Context, cfg Config, refreshToken string) (*Token, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("no refresh token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	return postToken(ctx, cfg, form)
}

// ── grants ─────────────────────────────────────────────────────────────────

func doClientCredentials(ctx context.Context, cfg Config) (*Token, error) {
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("client_credentials: tokenUrl required")
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}
	return postToken(ctx, cfg, form)
}

func doPassword(ctx context.Context, cfg Config) (*Token, error) {
	if cfg.TokenURL == "" || cfg.Username == "" {
		return nil, fmt.Errorf("password grant: tokenUrl + username required")
	}
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", cfg.Username)
	form.Set("password", cfg.Password)
	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	return postToken(ctx, cfg, form)
}

func doAuthCode(ctx context.Context, cfg Config, openBrowser func(string) error) (*Token, error) {
	if cfg.AuthURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("authorization_code: authUrl, tokenUrl, clientId required")
	}
	if openBrowser == nil {
		return nil, fmt.Errorf("authorization_code: no browser opener configured")
	}

	// Pick a transient port for the callback unless the caller fixed one.
	redirect := cfg.RedirectURI
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("callback listener: %w", err)
	}
	defer listener.Close()
	if redirect == "" {
		redirect = fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)
	}

	state := randomString(32)
	verifier := randomString(64) // PKCE
	challenge := sha256Base64URL(verifier)

	// Build the authorize URL.
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", state)
	if cfg.Scope != "" {
		q.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		q.Set("audience", cfg.Audience)
	}
	usePKCE := cfg.UsePKCE || cfg.UsePKCE == false // default on (typical client)
	_ = usePKCE
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	authorizeURL := cfg.AuthURL
	if strings.Contains(authorizeURL, "?") {
		authorizeURL += "&" + q.Encode()
	} else {
		authorizeURL += "?" + q.Encode()
	}

	// Spin up a one-shot handler that captures ?code= from the callback.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	once := sync.Once{}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			gotState := r.URL.Query().Get("state")
			if gotState != state {
				errCh <- fmt.Errorf("state mismatch (csrf): got %q, want %q", gotState, state)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("state mismatch — close this tab."))
				return
			}
			if e := r.URL.Query().Get("error"); e != "" {
				errCh <- fmt.Errorf("authorize: %s — %s", e, r.URL.Query().Get("error_description"))
				_, _ = w.Write([]byte("authorization failed — close this tab."))
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				errCh <- fmt.Errorf("no code in callback")
				return
			}
			_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>reqost</title><body style="font-family:system-ui;padding:40px"><h2>Authorized ✓</h2><p>You can close this tab and return to reqost.</p></body>`))
			codeCh <- code
		})
	})

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	if err := openBrowser(authorizeURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	// Wait for the user.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case code := <-codeCh:
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("code_verifier", verifier)
		return postToken(ctx, cfg, form)
	}
}

// ── token exchange ─────────────────────────────────────────────────────────

func postToken(ctx context.Context, cfg Config, form url.Values) (*Token, error) {
	if cfg.ClientAuthIn == "body" {
		if cfg.ClientID != "" {
			form.Set("client_id", cfg.ClientID)
		}
		if cfg.ClientSecret != "" {
			form.Set("client_secret", cfg.ClientSecret)
		}
	} else if cfg.ClientID != "" {
		form.Set("client_id", cfg.ClientID) // some servers require it in body even with Basic
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if cfg.ClientAuthIn != "body" && cfg.ClientID != "" && cfg.ClientSecret != "" {
		req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string      `json:"access_token"`
		TokenType    string      `json:"token_type"`
		RefreshToken string      `json:"refresh_token"`
		Scope        string      `json:"scope"`
		ExpiresIn    json.Number `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse token response: %w (body=%s)", err, string(body))
	}
	t := &Token{
		AccessToken:  raw.AccessToken,
		TokenType:    raw.TokenType,
		RefreshToken: raw.RefreshToken,
		Scope:        raw.Scope,
	}
	if exp, err := raw.ExpiresIn.Int64(); err == nil && exp > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(exp) * time.Second)
	}
	if t.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned no access_token (body=%s)", string(body))
	}
	if t.TokenType == "" {
		t.TokenType = "Bearer"
	}
	return t, nil
}

// ── utils ──────────────────────────────────────────────────────────────────

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

func sha256Base64URL(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
