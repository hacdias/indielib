package indieauth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverEndpointsNoToken(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</auth>; rel="authorization_endpoint"`)
					_, _ = w.Write([]byte(`<html></html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "", endpoints.Token)
	}
}

func TestDiscoverEndpointsNoAuthorization(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write([]byte(`<html></html>`))
				}),
			},
		},
	)

	_, err := client.DiscoverEndpoints("https://example.org/")
	assert.EqualValues(t, err, ErrNoEndpointFound)
}

func TestDiscoverEndpointsHTTPLink(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</auth>; rel="authorization_endpoint", </token>; rel="token_endpoint"`)
					_, _ = w.Write([]byte(`<html></html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "https://example.org/token", endpoints.Token)
	}
}

func TestDiscoverEndpointsBody(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write([]byte(`<html>
						<head>
							<link rel="authorization_endpoint" href="/auth">
							<link rel="token_endpoint" href="/token">
						</head>
					</html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "https://example.org/token", endpoints.Token)
	}
}

func TestDiscoverEndpointsMixed(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</auth>; rel="authorization_endpoint"`)
					_, _ = w.Write([]byte(`<html>
						<head>
							<link rel="token_endpoint" href="/token">
						</head>
					</html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "https://example.org/token", endpoints.Token)
	}
}

func TestDiscoverEndpointsUsesFirst(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</auth>; rel="authorization_endpoint"`)
					_, _ = w.Write([]byte(`<html>
						<head>
							<link rel="authorization_endpoint" href="/not/first">
							<link rel="token_endpoint" href="/token">
						</head>
					</html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "https://example.org/token", endpoints.Token)
	}
}

func TestDiscoverEndpointsHead(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</auth>; rel="authorization_endpoint"`)

					if r.Method == http.MethodHead {
						w.WriteHeader(http.StatusOK)
					} else {
						_, _ = w.Write([]byte(`<html>
							<head>
								<link rel="authorization_endpoint" href="/not/first">
								<link rel="token_endpoint" href="/token">
							</head>
						</html>`))
					}
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "https://example.org/token", endpoints.Token)
	}
}

func TestDiscoverEndpointExists(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `<.>; rel="test"`)
					w.WriteHeader(http.StatusOK)
				}),
			},
		},
	)

	endpoint, err := client.DiscoverEndpoint("https://example.org/", "test")
	assert.Nil(t, err)
	assert.EqualValues(t, "https://example.org/", endpoint)
}

func TestDiscoverEndpointNotExists(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusOK)
				}),
			},
		},
	)

	_, err := client.DiscoverEndpoint("https://example.org/", "test")
	assert.EqualValues(t, err, ErrNoEndpointFound)
}

func TestDiscoverEndpointHeadErrors(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")

					if r.Method == http.MethodHead {
						w.WriteHeader(http.StatusInternalServerError)
					} else {
						_, _ = w.Write([]byte(`<html>
							<head>\
								<link rel="authorization_endpoint" href="/auth">
							</head>
						</html>`))
					}
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "", endpoints.Token)
	}
}

func TestDiscoverEndpointGetErrors(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")

					if r.Method == http.MethodHead {
						w.Header().Set("Link", `</auth>; rel="authorization_endpoint"`)
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.Authorization)
		assert.EqualValues(t, "", endpoints.Token)
	}
}

func TestDiscoverEndpointHeadGetError(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
				}),
			},
		},
	)

	endpoints, err := client.DiscoverEndpoints("https://example.org/")
	assert.NotNil(t, err)
	assert.Nil(t, endpoints)
}

type handlerRoundTripper struct {
	http.RoundTripper
	handler http.Handler
}

func (rt *handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.handler != nil {
		// Fake request with handler
		rec := httptest.NewRecorder()
		rt.handler.ServeHTTP(rec, req)
		resp := rec.Result()
		// Copy request to response
		resp.Request = req
		return resp, nil
	}
	return nil, errors.New("no handler")
}
