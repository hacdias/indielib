package indieauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverMetadata(t *testing.T) {
	client := NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/metadata" {
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						_, _ = w.Write([]byte(`{
							"issuer": "https://example.org/",
							"authorization_endpoint": "https://example.org/auth",
							"token_endpoint": "https://example.org/token"
						}`))
						return
					}

					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Header().Set("Link", `</metadata>; rel="indieauth-metadata"`)
					_, _ = w.Write([]byte(`<html></html>`))
				}),
			},
		},
	)

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/", endpoints.Issuer)
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataNoToken(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataNoAuthorization(t *testing.T) {
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

	_, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.EqualValues(t, err, ErrNoEndpointFound)
}

func TestDiscoverMetadataHTTPLink(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataBody(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataMixed(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataUsesFirst(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
	}
}

func TestDiscoverMetadataHead(t *testing.T) {
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "https://example.org/token", endpoints.TokenEndpoint)
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

	endpoint, err := client.DiscoverLinkEndpoint(context.Background(), "https://example.org/", "test")
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

	_, err := client.DiscoverLinkEndpoint(context.Background(), "https://example.org/", "test")
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "", endpoints.TokenEndpoint)
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
	assert.Nil(t, err)
	assert.NotNil(t, endpoints)
	if endpoints != nil {
		assert.EqualValues(t, "https://example.org/auth", endpoints.AuthorizationEndpoint)
		assert.EqualValues(t, "", endpoints.TokenEndpoint)
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

	endpoints, err := client.DiscoverMetadata(context.Background(), "https://example.org/")
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

func TestDiscoverApplicationMetadata(t *testing.T) {
	for _, testCase := range []struct {
		clientID string
		body     string
		expected ApplicationMetadata
	}{
		{
			clientID: "https://example.com/",
			body: `
		<div class="h-x-app">
			<img src="/logo.png" class="u-logo">
			<a href="/" class="u-url p-name">Example App</a>
		</div>`,
			expected: ApplicationMetadata{
				Name: "Example App",
				Logo: "https://example.com/logo.png",
				URL:  "https://example.com/",
			},
		},
		{
			clientID: "https://example.com/",
			body: `
		<div class="h-app">
			<img src="/logo.png" class="u-logo">
			<a href="/" class="u-url p-name">Example App</a>
			<a href="/" class="u-url p-name">Example App 2</a>
			<p class="p-author">Example Author</a>
		</div>`,
			expected: ApplicationMetadata{
				Name:   "Example App",
				Logo:   "https://example.com/logo.png",
				URL:    "https://example.com/",
				Author: "Example Author",
			},
		},
		{
			clientID: "https://example.com/",
			body: `
		<div class="something-else h-app">
			<img src="/logo.png" class="u-photo">
			<a href="/" class="u-url p-name">Example App</a>
		</div>`,
			expected: ApplicationMetadata{
				Name: "Example App",
				Logo: "https://example.com/logo.png",
				URL:  "https://example.com/",
			},
		},
		{
			clientID: "https://example.com/",
			body: `
		<div class="something-else h-app">
			<img src="/logo.png" class="u-logo p-name" alt="Example App">
		</div>`,
			expected: ApplicationMetadata{
				Name: "Example App",
				Logo: "https://example.com/logo.png",
			},
		},
	} {
		httpClient := &http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.String() != testCase.clientID {
						w.WriteHeader(http.StatusExpectationFailed)
						return
					}
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write([]byte(testCase.body))
				}),
			},
		}

		server := NewServer(false, httpClient)
		data, err := server.DiscoverApplicationMetadata(context.Background(), testCase.clientID)
		assert.Nil(t, err)
		assert.Equal(t, testCase.expected.Name, data.Name)
		assert.Equal(t, testCase.expected.Logo, data.Logo)
		assert.Equal(t, testCase.expected.URL, data.URL)
		assert.Equal(t, testCase.expected.Summary, data.Summary)
		assert.Equal(t, testCase.expected.Author, data.Author)
	}
}

func TestDiscoverApplicationMetadataError(t *testing.T) {
	for _, testCase := range []struct {
		clientID string
		body     string
		err      error
	}{
		{
			clientID: "https://example.com/",
			body:     `<div class="h-x-app"><p class="p-name"></p></div>`,
			err:      ErrNoApplicationMetadata,
		},
		{
			clientID: "https://example.com/",
			body:     `<div class="h-app"><p class="p-name"></p></div>`,
			err:      ErrNoApplicationMetadata,
		},
		{
			clientID: "https://example.com/",
			body:     `<div class="h-not-app"><p class="p-name">Example App</p></div>`,
			err:      ErrNoApplicationMetadata,
		},
		{
			clientID: "https://example.com/",
			body:     ``,
			err:      ErrNoApplicationMetadata,
		},
	} {
		httpClient := &http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.String() != testCase.clientID {
						w.WriteHeader(http.StatusExpectationFailed)
						return
					}
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write([]byte(testCase.body))
				}),
			},
		}

		server := NewServer(false, httpClient)
		data, err := server.DiscoverApplicationMetadata(context.Background(), testCase.clientID)
		assert.ErrorIs(t, err, testCase.err)
		assert.Nil(t, data)
	}
}
