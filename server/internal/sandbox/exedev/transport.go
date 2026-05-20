package exedev

import (
	"net/http"
	"net/url"
)

type vmHTTPTransport struct {
	base   http.RoundTripper
	host   string
	token  string
	scheme string
}

func (t *vmHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	urlCopy := *req.URL
	urlCopy.Scheme = t.scheme
	urlCopy.Host = t.host
	clone.URL = &urlCopy
	clone.Host = t.host
	clone.Header.Set("X-Exedev-Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func (t *vmHTTPTransport) Headers() http.Header {
	headers := make(http.Header)
	headers.Set("X-Exedev-Authorization", "Bearer "+t.token)
	return headers
}

func (t *vmHTTPTransport) WebSocketURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Scheme = "wss"
	u.Host = t.host
	return u.String()
}
