package httpclient

import (
	"net/http/cookiejar"
	"net/url"
)

// Cookie is a flattened view of a stored cookie for the UI.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// Cookies returns the cookies the jar would send to rawurl. The stdlib jar only
// exposes name/value via Cookies(), so domain/path are filled from the URL.
func (c *Client) Cookies(rawurl string) []Cookie {
	u, err := url.Parse(rawurl)
	if err != nil || c.jar == nil {
		return []Cookie{}
	}
	out := []Cookie{}
	for _, ck := range c.jar.Cookies(u) {
		out = append(out, Cookie{Name: ck.Name, Value: ck.Value, Domain: u.Host, Path: "/"})
	}
	return out
}

// ClearCookies drops all stored cookies by swapping in a fresh jar.
func (c *Client) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	c.jar = jar
}
