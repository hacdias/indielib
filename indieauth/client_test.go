package indieauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	t.Run("Fails If Cannot Discover Metadata", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			"https://example.com/",
			"https://example.com/callback",
			&http.Client{
				Transport: &handlerRoundTripper{
					handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
					}),
				},
			},
		)

		_, _, err := client.Authenticate(context.Background(), "https://example.com/", "profile post")
		require.Error(t, err)
	})

	t.Run("Successful Request", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			"https://example.com/",
			"https://example.com/callback",
			&http.Client{
				Transport: &handlerRoundTripper{
					handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/metadata" {
							w.Header().Set("Content-Type", "application/json; charset=utf-8")
							_, _ = w.Write([]byte(`{
								"issuer": "https://example.com/",
								"authorization_endpoint": "https://example.com/auth",
								"token_endpoint": "https://example.com/token"
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

		authInfo, redirect, err := client.Authenticate(context.Background(), "https://example.com/", "profile post")
		require.Nil(t, err)
		require.NotNil(t, authInfo)
		require.Equal(t, "https://example.com/", authInfo.Me)
		require.Equal(t, "https://example.com/", authInfo.Issuer)
		require.Equal(t, "https://example.com/auth", authInfo.AuthorizationEndpoint)
		require.Equal(t, "https://example.com/token", authInfo.TokenEndpoint)
		require.NotEmpty(t, authInfo.State)
		require.NotEmpty(t, authInfo.CodeVerifier)

		redirectURL, err := url.Parse(redirect)
		require.NoError(t, err)
		require.Equal(t, "profile post", redirectURL.Query().Get("scope"))
		require.Equal(t, "S256", redirectURL.Query().Get("code_challenge_method"))
		require.Equal(t, authInfo.State, redirectURL.Query().Get("state"))
		require.Equal(t, s256Challenge(authInfo.CodeVerifier), redirectURL.Query().Get("code_challenge"))
	})
}

func TestValidateCallback(t *testing.T) {
	iac := NewClient("https://example.com/", "https://example.com/callback", nil)

	for _, testCase := range []struct {
		authInfo      *AuthInfo
		code          string
		state         string
		iss           string
		expectedError error
		expectedCode  string
	}{
		{&AuthInfo{}, "", "", "", ErrCodeNotFound, ""},
		{&AuthInfo{}, "code", "", "", ErrStateNotFound, ""},
		{&AuthInfo{State: "other state"}, "code", "state", "", ErrInvalidState, ""},
		{&AuthInfo{State: "state", Metadata: Metadata{Issuer: "https://example.com/"}}, "code", "state", "https://example.org/", ErrInvalidIssuer, ""},
		{&AuthInfo{State: "state", Metadata: Metadata{Issuer: "https://example.org/"}}, "code", "state", "https://example.org/", nil, "code"},
	} {
		query := url.Values{}
		query.Set("code", testCase.code)
		query.Set("state", testCase.state)
		query.Set("iss", testCase.iss)
		r := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), nil)
		code, err := iac.ValidateCallback(testCase.authInfo, r)
		assert.ErrorIs(t, err, testCase.expectedError)
		assert.Equal(t, code, testCase.expectedCode)
	}
}

func TestProfileFromToken(t *testing.T) {
	t.Parallel()

	completeProfileTokenData := map[string]interface{}{
		"me": "https://example.com/",
		"profile": map[string]interface{}{
			"name":  "John Smith",
			"url":   "https://example.com/",
			"photo": "https://example.com/profile.png",
			"email": "noreply@example.com",
		},
	}

	completeProfile := &Profile{Me: "https://example.com/"}
	completeProfile.Profile.Name = "John Smith"
	completeProfile.Profile.URL = "https://example.com/"
	completeProfile.Profile.Photo = "https://example.com/profile.png"
	completeProfile.Profile.Email = "noreply@example.com"

	for _, testCase := range []struct {
		tokenData map[string]interface{}
		profile   *Profile
	}{
		{nil, nil},
		{map[string]interface{}{}, nil},
		{map[string]interface{}{"me": "https://example.com/"}, &Profile{Me: "https://example.com/"}},
		{completeProfileTokenData, completeProfile},
	} {
		token := new(oauth2.Token).WithExtra(testCase.tokenData)
		profile := ProfileFromToken(token)
		assert.EqualValues(t, testCase.profile, profile)
	}
}

func TestGetToken(t *testing.T) {
	code := "abc123"
	codeVerifier := "123xyz"

	client := NewClient(
		"https://example.com/",
		"https://example.com/callback",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/token" && r.Method == http.MethodPost {
						if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
							http.Error(w, "wrong Content-Type header", http.StatusBadRequest)
							return
						}

						err := r.ParseForm()
						if err != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}

						t.Log(r.Method, r.URL.Path, r.Form, r.Header)

						if r.Form.Get("grant_type") != "authorization_code" {
							http.Error(w, "wrong grant_type", http.StatusBadRequest)
							return
						}

						if r.Form.Get("redirect_uri") != "https://example.com/callback" {
							http.Error(w, "wrong redirect_uri", http.StatusBadRequest)
							return
						}

						if r.Form.Get("client_id") != "https://example.com/" {
							http.Error(w, "wrong client_id", http.StatusBadRequest)
							return
						}

						if r.Form.Get("code") != code {
							http.Error(w, "wrong code", http.StatusBadRequest)
							return
						}

						if r.Form.Get("code_verifier") != codeVerifier {
							http.Error(w, "wrong code_verifier", http.StatusBadRequest)
							return
						}

						bytes, err := json.Marshal(map[string]interface{}{
							"me":           "https://example.com/john",
							"access_token": "token",
						})
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}

						w.Header().Set("Content-Type", "applications/json; charset=utf-8")
						_, _ = w.Write(bytes)
						return
					}

					w.WriteHeader(http.StatusMethodNotAllowed)
				}),
			},
		},
	)

	authInfo := &AuthInfo{
		Metadata: Metadata{
			TokenEndpoint: "https://example.com/token",
		},
		CodeVerifier: codeVerifier,
	}

	token, conf, err := client.GetToken(context.Background(), authInfo, code)
	require.Nil(t, err)
	require.NotNil(t, conf)
	require.NotNil(t, token)
	require.Equal(t, "token", token.AccessToken)
}

func TestFetchProfile(t *testing.T) {
	code := "abc123"
	codeVerifier := "123xyz"
	originalProfile := &Profile{
		Me: "https://example.com/john",
	}

	client := NewClient(
		"https://example.com/",
		"https://example.com/callback",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/auth" && r.Method == http.MethodPost {
						if r.Header.Get("Accept") != "application/json" {
							http.Error(w, "wrong Accept header", http.StatusBadRequest)
							return
						}

						if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
							http.Error(w, "wrong Content-Type header", http.StatusBadRequest)
							return
						}

						err := r.ParseForm()
						if err != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}

						if r.Form.Get("grant_type") != "authorization_code" {
							http.Error(w, "wrong grant_type", http.StatusBadRequest)
							return
						}

						if r.Form.Get("redirect_uri") != "https://example.com/callback" {
							http.Error(w, "wrong redirect_uri", http.StatusBadRequest)
							return
						}

						if r.Form.Get("client_id") != "https://example.com/" {
							http.Error(w, "wrong client_id", http.StatusBadRequest)
							return
						}

						if r.Form.Get("code") != code {
							http.Error(w, "wrong code", http.StatusBadRequest)
							return
						}

						if r.Form.Get("code_verifier") != codeVerifier {
							http.Error(w, "wrong code_verifier", http.StatusBadRequest)
							return
						}

						bytes, err := json.Marshal(originalProfile)
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}

						w.Header().Set("Content-Type", "applications/json; charset=utf-8")
						_, _ = w.Write(bytes)
						return
					}

					w.WriteHeader(http.StatusMethodNotAllowed)
				}),
			},
		},
	)

	authInfo := &AuthInfo{
		Metadata: Metadata{
			AuthorizationEndpoint: "https://example.com/auth",
		},
		CodeVerifier: codeVerifier,
	}

	profile, err := client.FetchProfile(context.Background(), authInfo, code)
	require.Nil(t, err)
	require.EqualValues(t, originalProfile, profile)
}
