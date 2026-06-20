package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// escapeQuotes escapes `"` so it survives Content-Disposition header values.
func escapeQuotes(s string) string { return strings.ReplaceAll(s, `"`, `\"`) }

// buildBody turns a request's body spec into an io.Reader plus the Content-Type
// to set (empty if none). GET/HEAD never carry a body. Variables are
// interpolated into every value.
func buildBody(req Request, method string, vars map[string]string) (io.Reader, string, error) {
	if method == http.MethodGet || method == http.MethodHead {
		return nil, "", nil
	}

	switch req.BodyType {
	case "", "none":
		return nil, "", nil

	case "raw":
		raw := interpolate(req.Body, vars)
		if raw == "" {
			return nil, "", nil
		}
		return strings.NewReader(raw), "", nil

	case "json":
		raw := interpolate(req.Body, vars)
		if raw == "" {
			return nil, "", nil
		}
		return strings.NewReader(raw), "application/json", nil

	case "urlencoded":
		form := url.Values{}
		for _, f := range req.FormFields {
			if !f.Enabled || f.Key == "" {
				continue
			}
			form.Add(interpolate(f.Key, vars), interpolate(f.Value, vars))
		}
		if len(form) == 0 {
			return nil, "", nil
		}
		return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil

	case "formdata":
		return buildMultipart(req.FormFields, vars)

	case "binary":
		// `body` holds an absolute path; stream it as octet-stream. We don't
		// load the whole file into memory.
		path := interpolate(req.Body, vars)
		if path == "" {
			return nil, "", nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, "", fmt.Errorf("binary body: open %s: %w", path, err)
		}
		return f, "application/octet-stream", nil

	default:
		return nil, "", fmt.Errorf("unsupported bodyType %q", req.BodyType)
	}
}

// buildMultipart builds a multipart/form-data body, streaming file fields from
// disk. The whole body is buffered (net/http needs a known length / replayable
// body anyway), bounded indirectly by the files chosen.
func buildMultipart(fields []FormField, vars map[string]string) (io.Reader, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	any := false

	for _, f := range fields {
		if !f.Enabled || f.Key == "" {
			continue
		}
		key := interpolate(f.Key, vars)
		ct := strings.TrimSpace(f.ContentType)
		if f.Type == "file" {
			path := interpolate(f.Value, vars)
			if path == "" {
				continue
			}
			file, err := os.Open(path)
			if err != nil {
				return nil, "", fmt.Errorf("open upload %s: %w", path, err)
			}
			var part io.Writer
			if ct != "" {
				h := textproto.MIMEHeader{}
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(key), escapeQuotes(filepath.Base(path))))
				h.Set("Content-Type", ct)
				part, err = mw.CreatePart(h)
			} else {
				part, err = mw.CreateFormFile(key, filepath.Base(path))
			}
			if err != nil {
				file.Close()
				return nil, "", err
			}
			if _, err := io.Copy(part, file); err != nil {
				file.Close()
				return nil, "", fmt.Errorf("copy upload %s: %w", path, err)
			}
			file.Close()
		} else {
			val := interpolate(f.Value, vars)
			if ct != "" {
				h := textproto.MIMEHeader{}
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(key)))
				h.Set("Content-Type", ct)
				part, err := mw.CreatePart(h)
				if err != nil {
					return nil, "", err
				}
				if _, err := io.WriteString(part, val); err != nil {
					return nil, "", err
				}
			} else if err := mw.WriteField(key, val); err != nil {
				return nil, "", err
			}
		}
		any = true
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	if !any {
		return nil, "", nil
	}
	return &buf, mw.FormDataContentType(), nil
}
