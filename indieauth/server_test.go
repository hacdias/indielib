package indieauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAuthorization(t *testing.T) {
	t.Parallel()

	t.Run("Non-Code Challenge Validation", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			requirePKCE   bool
			responseType  string
			clientID      string
			redirectURI   string
			expectedError error
		}{
			{false, "token", "", "", ErrInvalidResponseType},
			{false, "code", "https://127.0.0.1:5050", "", ErrInvalidClientIdentifier},
			{false, "", "https://example.com/", "https://example.com/callback", nil},
			{false, "code", "https://example.com/", "https://example.com/callback", nil},
			{false, "code", "https://example.com/", "this ain't a URL", ErrInvalidRedirectURI},
			{true, "code", "https://example.com/", "https://example.com/callback", ErrPKCERequired},
		} {
			ias := NewServer(testCase.requirePKCE, nil)

			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Form = url.Values{}
			r.Form.Set("response_type", testCase.responseType)
			r.Form.Set("client_id", testCase.clientID)
			r.Form.Set("redirect_uri", testCase.redirectURI)
			r.Form.Set("state", "abc123")

			authReq, err := ias.ParseAuthorization(r)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, authReq)
				assert.Equal(t, authReq.ClientID, testCase.clientID)
				assert.Equal(t, authReq.RedirectURI, testCase.redirectURI)
				assert.Equal(t, authReq.State, "abc123")
			} else {
				assert.Nil(t, authReq)
				assert.ErrorIs(t, err, testCase.expectedError)
			}
		}
	})

	t.Run("Code Challenge Validation", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			requirePKCE         bool
			codeChallenge       string
			codeChallengeMethod string
			scope               string
			expectedError       error
		}{
			{true, strings.Repeat("a", 10), "plain", "profile", ErrWrongCodeChallengeLength},
			{true, strings.Repeat("a", 43), "plain", "profile", nil},
			{true, strings.Repeat("a", 128), "plain", "profile", nil},
			{true, strings.Repeat("a", 130), "plain", "profile", ErrWrongCodeChallengeLength},
			{true, strings.Repeat("a", 100), "unknown", "profile", ErrInvalidCodeChallengeMethod},
			{false, strings.Repeat("a", 10), "plain", "profile", ErrWrongCodeChallengeLength},
			{false, strings.Repeat("a", 43), "plain", "profile", nil},
			{false, strings.Repeat("a", 128), "plain", "profile", nil},
			{false, strings.Repeat("a", 130), "plain", "profile", ErrWrongCodeChallengeLength},
			{false, strings.Repeat("a", 100), "unknown", "profile", ErrInvalidCodeChallengeMethod},
		} {
			ias := NewServer(testCase.requirePKCE, nil)

			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Form = url.Values{}
			r.Form.Set("response_type", "code")
			r.Form.Set("client_id", "https://example.com/")
			r.Form.Set("redirect_uri", "https://example.com/callback")
			r.Form.Set("code_challenge", testCase.codeChallenge)
			r.Form.Set("code_challenge_method", testCase.codeChallengeMethod)
			r.Form.Set("state", "abc123")
			r.Form.Set("scope", testCase.scope)

			authReq, err := ias.ParseAuthorization(r)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				assert.NotNil(t, authReq)
				assert.Equal(t, authReq.ClientID, "https://example.com/")
				assert.Equal(t, authReq.RedirectURI, "https://example.com/callback")
				assert.Equal(t, authReq.CodeChallenge, testCase.codeChallenge)
				assert.Equal(t, authReq.CodeChallengeMethod, testCase.codeChallengeMethod)
				assert.Equal(t, authReq.State, "abc123")
			} else {
				assert.Nil(t, authReq)
				assert.ErrorIs(t, err, testCase.expectedError)
			}
		}
	})

	t.Run("Scopes", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Form = url.Values{}
		r.Form.Set("client_id", "https://example.com/")
		r.Form.Set("redirect_uri", "https://example.com/callback")
		r.Form.Set("scope", " create  update ")

		authReq, err := NewServer(false, nil).ParseAuthorization(r)
		assert.NoError(t, err)
		assert.Equal(t, []string{"create", "update"}, authReq.Scopes)
	})

	t.Run("Redirect URI Discovery", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/app/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"redirect_uris": ["https://other.example/callback"]}`))
		})
		mux.HandleFunc("/html/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Link", `<https://header.example/callback>; rel="redirect_uri"`)
			_, _ = w.Write([]byte(`<html><head>
				<link rel="redirect_uri" href="https://other.example/callback">
				<link rel="redirect_uri" href="https://another.example/callback">
			</head></html>`))
		})
		mux.HandleFunc("/html-empty/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html></html>`))
		})
		mux.HandleFunc("/missing/", http.NotFound)

		for _, testCase := range []struct {
			clientID      string
			redirectURI   string
			expectedError error
		}{
			{"https://example.com/app/", "https://other.example/callback", nil},
			{"https://example.com/app/", "https://other.example/nope", ErrInvalidRedirectURI},
			{"https://example.com/app/", "http://example.com/app/callback", ErrInvalidRedirectURI},
			{"https://example.com/html/", "https://other.example/callback", nil},
			{"https://example.com/html/", "https://another.example/callback", nil},
			{"https://example.com/html/", "https://header.example/callback", nil},
			{"https://example.com/html/", "https://other.example/nope", ErrInvalidRedirectURI},
			{"https://example.com/html-empty/", "https://other.example/callback", ErrInvalidRedirectURI},
			{"https://example.com/missing/", "https://other.example/callback", ErrInvalidRedirectURI},
			{"http://127.0.0.1:8080/app/", "https://other.example/callback", ErrLoopbackClientID},
			{"http://localhost:8080/app/", "https://other.example/callback", ErrLoopbackClientID},
		} {
			ias := NewServer(false, &http.Client{Transport: &handlerRoundTripper{handler: mux}})

			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Form = url.Values{}
			r.Form.Set("response_type", "code")
			r.Form.Set("client_id", testCase.clientID)
			r.Form.Set("redirect_uri", testCase.redirectURI)

			authReq, err := ias.ParseAuthorization(r)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, authReq.RedirectURI, testCase.redirectURI)
			} else {
				assert.Nil(t, authReq)
				assert.ErrorIs(t, err, testCase.expectedError)
			}
		}
	})
}

func TestValidateTokenExchange(t *testing.T) {
	t.Parallel()

	t.Run("Non-Code Challenge Validation", func(t *testing.T) {
		t.Parallel()

		authReq := &AuthenticationRequest{
			ClientID:            "https://example.com/",
			RedirectURI:         "https://example.com/callback",
			CodeChallenge:       strings.Repeat("a", 50),
			CodeChallengeMethod: "plain",
		}

		for _, testCase := range []struct {
			grantType     string
			clientID      string
			redirectURI   string
			expectedError error
		}{
			{"authorization_code", "https://example.com/", "https://example.com/callback", nil},
			{"", "https://example.com/", "https://example.com/callback", nil},
			{"client_credentials", "https://example.com/", "https://example.com/callback", ErrInvalidGrantType},
			{"authorization_code", "https://example.org/", "https://example.com/callback", ErrNoMatchClientID},
			{"authorization_code", "https://example.com/", "https://example.com/callback2", ErrNoMatchRedirectURI},
		} {
			ias := NewServer(true, nil)
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Form = url.Values{}
			r.Form.Set("grant_type", testCase.grantType)
			r.Form.Set("client_id", testCase.clientID)
			r.Form.Set("redirect_uri", testCase.redirectURI)
			r.Form.Set("code_verifier", strings.Repeat("a", 50))

			err := ias.ValidateTokenExchange(authReq, r)
			assert.ErrorIs(t, err, testCase.expectedError)
		}
	})

	t.Run("Code Challenge Validation", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			requirePKCE         bool
			codeChallenge       string
			codeChallengeMethod string
			codeVerifier        string
			expectedError       error
		}{
			{true, "", "", "", ErrPKCERequired},
			{false, "", "", "", nil},
			{false, strings.Repeat("a", 50), "plain", strings.Repeat("a", 50), nil},
			{true, strings.Repeat("a", 50), "plain", strings.Repeat("a", 50), nil},
			{false, strings.Repeat("a", 50), "plain", "", ErrWrongCodeVerifierLength},
			{false, strings.Repeat("a", 10), "plain", strings.Repeat("a", 50), ErrWrongCodeChallengeLength},
			{false, strings.Repeat("a", 50), "unknown", strings.Repeat("a", 50), ErrInvalidCodeChallengeMethod},
			{false, strings.Repeat("a", 50), "plain", strings.Repeat("a", 51), ErrCodeChallengeFailed},
		} {
			ias := NewServer(testCase.requirePKCE, nil)

			authReq := &AuthenticationRequest{
				ClientID:            "https://example.com/",
				RedirectURI:         "https://example.com/callback",
				CodeChallenge:       testCase.codeChallenge,
				CodeChallengeMethod: testCase.codeChallengeMethod,
			}

			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Form = url.Values{}
			r.Form.Set("grant_type", "authorization_code")
			r.Form.Set("client_id", authReq.ClientID)
			r.Form.Set("redirect_uri", authReq.RedirectURI)
			r.Form.Set("code_verifier", testCase.codeVerifier)

			err := ias.ValidateTokenExchange(authReq, r)
			assert.ErrorIs(t, err, testCase.expectedError)
		}
	})
}
