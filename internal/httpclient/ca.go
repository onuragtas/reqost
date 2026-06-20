package httpclient

import (
	"crypto/x509"
	"os"
	"sync"
)

// loadCAPool reads a PEM bundle from disk and merges it with the system root
// pool. Returns nil on any error / empty file, signalling "use the default
// system pool instead". Results are cached by absolute path so the disk isn't
// re-read for every request — the user has to restart for a new bundle, which
// matches the surface most TLS tooling exposes.
var (
	caMu    sync.Mutex
	caCache = map[string]*x509.CertPool{}
)

func loadCAPool(path string) *x509.CertPool {
	if path == "" {
		return nil
	}
	caMu.Lock()
	defer caMu.Unlock()
	if p, ok := caCache[path]; ok {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		caCache[path] = nil
		return nil
	}
	// Start from system roots so the user's bundle adds to rather than
	// replaces the trust set — matches `SSL_CERT_FILE` semantics.
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data) {
		caCache[path] = nil
		return nil
	}
	caCache[path] = pool
	return pool
}

// ResetCACache is exposed for tests that need to bust the cache between runs.
func ResetCACache() {
	caMu.Lock()
	caCache = map[string]*x509.CertPool{}
	caMu.Unlock()
}
