package httpclient

import (
	"net/http"
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

// SetCookie adds or overwrites a cookie. Caller supplies the URL the cookie
// is scoped to; the jar derives domain/path from there. Used by the UI's
// "manual cookie edit" affordance.
func (c *Client) SetCookie(rawurl, name, value string) error {
	u, err := url.Parse(rawurl)
	if err != nil || c.jar == nil {
		return err
	}
	c.jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
	return nil
}

// DeleteCookie unsets a cookie by name within the URL's scope. The stdlib jar
// has no Delete API — we set the cookie to an empty value with MaxAge=-1.
func (c *Client) DeleteCookie(rawurl, name string) error {
	u, err := url.Parse(rawurl)
	if err != nil || c.jar == nil {
		return err
	}
	c.jar.SetCookies(u, []*http.Cookie{{Name: name, Value: "", Path: "/", MaxAge: -1}})
	return nil
}
