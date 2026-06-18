package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"reqost/internal/oauth2"
)

// OAuthService is the JS-facing wrapper around internal/oauth2. It caches the
// last-issued token keyed by the auth config so back-to-back requests (and
// scripts pm.sendRequest()) reuse the same access token until it's near
// expiry, at which point Refresh runs transparently.
type OAuthService struct {
	app *application.App

	mu    sync.Mutex
	cache map[string]*oauth2.Token // key = cacheKey(config)
}

func NewOAuthService() *OAuthService {
	return &OAuthService{cache: map[string]*oauth2.Token{}}
}

func (s *OAuthService) setApp(a *application.App) { s.app = a }

// GetToken returns a non-expired token for the given config, hitting the cache
// when possible. Always returns the full token (frontend writes it onto the
// active environment or request auth as the user prefers).
func (s *OAuthService) GetToken(cfg oauth2.Config) (*oauth2.Token, error) {
	key := cacheKey(cfg)

	s.mu.Lock()
	cached := s.cache[key]
	s.mu.Unlock()
	if cached != nil && !nearExpiry(cached) {
		return cached, nil
	}
	if cached != nil && cached.RefreshToken != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		t, err := oauth2.Refresh(ctx, cfg, cached.RefreshToken)
		if err == nil {
			s.put(key, t)
			return t, nil
		}
		// Fall through to a full re-auth if refresh failed.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	t, err := oauth2.Get(ctx, cfg, s.openBrowser)
	if err != nil {
		return nil, err
	}
	s.put(key, t)
	return t, nil
}

// ClearTokens drops every cached token. Useful after a sign-out flow.
func (s *OAuthService) ClearTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = map[string]*oauth2.Token{}
}

func (s *OAuthService) put(key string, t *oauth2.Token) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = t
}

func (s *OAuthService) openBrowser(u string) error {
	if s.app == nil || s.app.Browser == nil {
		return nil
	}
	return s.app.Browser.OpenURL(u)
}

// cacheKey ties a token to the config that produced it (same client id +
// token URL + grant + scope = same token).
func cacheKey(c oauth2.Config) string {
	return string(c.Grant) + "|" + c.TokenURL + "|" + c.ClientID + "|" + c.Scope + "|" + c.Audience + "|" + c.Username
}

func nearExpiry(t *oauth2.Token) bool {
	if t == nil || t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}
