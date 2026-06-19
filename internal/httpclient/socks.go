package httpclient

import (
	"context"
	"net"
	"net/url"

	"golang.org/x/net/proxy"
)

// socks5Dialer returns a DialContext that routes every connection through a
// SOCKS5 proxy at `host` (host:port). Authentication, if present in the URL,
// is forwarded.
func socks5Dialer(host string, auth *proxy.Auth) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	d, err := proxy.SOCKS5("tcp", host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}
	contextDialer, ok := d.(proxy.ContextDialer)
	if ok {
		return contextDialer.DialContext, nil
	}
	// Older xnet versions don't implement ContextDialer; wrap.
	return func(_ context.Context, network, addr string) (net.Conn, error) {
		return d.Dial(network, addr)
	}, nil
}

// socks5Auth pulls username/password out of a URL's userinfo (if any).
func socks5Auth(u *url.URL) (*proxy.Auth, bool) {
	if u.User == nil {
		return nil, false
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	if user == "" {
		return nil, false
	}
	return &proxy.Auth{User: user, Password: pass}, true
}
