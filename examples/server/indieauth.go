package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.hacdias.com/indielib/indieauth"
)

type authorization struct {
	req  *indieauth.AuthenticationRequest
	time time.Time
}

// isExpired checks if the authorization is expired. It should be a reasonably
// short lived, as the authorization code should be redeemed for a token at the
// token endpoint, or the user profile at the authorization endpoint.
func (ar *authorization) isExpired() bool {
	return time.Since(ar.time) > time.Minute*10
}

// storeAuthorization stores the authorization request and returns a code for it.
// Something such as JWT tokens could be used in a production environment.
func (s *server) storeAuthorization(req *indieauth.AuthenticationRequest) string {
	s.authorizationsMu.Lock()
	defer s.authorizationsMu.Unlock()

	code := randomString()
	s.authorizations[code] = &authorization{
		req:  req,
		time: time.Now(),
	}

	return code
}

// getAuthorization retrieves the authorization request corresponding to the code.
// If it does not exist, or is expired, returns nil.
func (s *server) getAuthorization(code string) *authorization {
	s.authorizationsMu.Lock()
	defer s.authorizationsMu.Unlock()

	t, ok := s.authorizations[code]
	if !ok {
		return nil
	}

	delete(s.authorizations, code)

	if t.isExpired() {
		return nil
	}
	return t
}

type token struct {
	time       time.Time
	scopes     []string
	expiration time.Time
}

func (tk *token) isExpired() bool {
	return tk.expiration.Before(time.Now())
}

// newToken creates a token for the given scope and returns its ID. In a production
// server, something like a JWT or a database entry could be created.
func (s *server) newToken(scopes []string) (string, time.Time) {
	s.tokensMu.Lock()
	defer s.tokensMu.Unlock()

	code := randomString()
	token := &token{
		scopes:     scopes,
		time:       time.Now(),
		expiration: time.Now().Add(time.Hour * 24),
	}
	s.tokens[code] = token

	return code, token.expiration
}

// getToken retrieves the token information for the given code. Deletes it if expired.
// In a production server, something like a JWT or a database entry could be created.
func (s *server) getToken(code string) *token {
	s.tokensMu.Lock()
	defer s.tokensMu.Unlock()

	t, ok := s.tokens[code]
	if !ok {
		return nil
	}

	if t.isExpired() {
		delete(s.tokens, code)
		return nil
	}
	return t
}

type contextKey string

const (
	scopesContextKey contextKey = "scopes"
)

// authorizationHandler handles the authorization endpoint, which can be used to:
//  1. GET - authorize a request.
//  2. POST - exchange the authorization code for the user's profile URL.
func (s *server) authorizationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.authorizationGetHandler(w, r)
		return
	}

	if r.Method == http.MethodPost {
		s.authorizationPostHandler(w, r)
		return
	}

	httpError(w, http.StatusMethodNotAllowed)
}

// authorizationGetHandler handles the GET method for the authorization endpoint.
func (s *server) authorizationGetHandler(w http.ResponseWriter, r *http.Request) {
	// In a production server, this page would usually be protected. In order for
	// the user to authorize this request, they must be authenticated. This could
	// be done in different ways: username/password, passkeys, etc.

	// Parse the authorization request.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Do a best effort attempt at fetching more information about the application
	// that we can show to the user. Not all applications provide this sort of
	// information.
	app, _ := s.ias.DiscoverApplicationMetadata(r.Context(), req.ClientID)

	// Here, we just display a small HTML document where the user has to press
	// to authorize this request. Please note that this template contains a form
	// where we dump all the request information. This makes it possible to reuse
	// [indieauth.Server.ParseAuthorization] when the user authorizes the request.
	serveHTML(w, "auth.html", map[string]any{
		"Request":     req,
		"Application": app,
	})
}

// authorizationPostHandler handles the POST method for the authorization endpoint.
func (s *server) authorizationPostHandler(w http.ResponseWriter, r *http.Request) {
	s.authorizationCodeExchange(w, r, false)
}

// tokenHandler handles the token endpoint. In our case, we only accept the default
// type which is exchanging an authorization code for a token.
func (s *server) tokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed)
		return
	}

	if r.Form.Get("grant_type") == "refresh_token" {
		// NOTE: this server does not implement refresh tokens.
		// https://indieauth.spec.indieweb.org/#refresh-tokens
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	s.authorizationCodeExchange(w, r, true)
}

type tokenResponse struct {
	Me          string `json:"me"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
}

// authorizationCodeExchange handles the authorization code exchange. It is used by
// both the authorization handler to exchange the code for the user's profile URL,
// and by the token endpoint, to exchange the code by a token.
func (s *server) authorizationCodeExchange(w http.ResponseWriter, r *http.Request, withToken bool) {
	if err := r.ParseForm(); err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	t := s.getAuthorization(r.Form.Get("code"))
	if t == nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid authorization")
		return
	}

	authRequest := &indieauth.AuthenticationRequest{
		ClientID:            t.req.ClientID,
		RedirectURI:         t.req.RedirectURI,
		CodeChallenge:       t.req.CodeChallenge,
		CodeChallengeMethod: t.req.CodeChallengeMethod,
	}

	err := s.ias.ValidateTokenExchange(authRequest, r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	response := &tokenResponse{
		Me: s.profileURL,
	}

	scope := t.req.Scopes

	if withToken {
		token, expiration := s.newToken(scope)
		response.AccessToken = token
		response.TokenType = "Bearer"
		response.ExpiresIn = int64(time.Until(expiration).Seconds())
		response.Scope = strings.Join(scope, " ")
	}

	// An actual server may want to include the "profile" in the response if the
	// scope "profile" is included.
	serveJSON(w, http.StatusOK, response)
}

func (s *server) authorizationAcceptHandler(w http.ResponseWriter, r *http.Request) {
	// Parse authorization information. This only works because our authorization page
	// includes all the required information. This can be done in other ways: database,
	// whether temporary or not, cookies, etc.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Generate a random code and persist the information associated to that code.
	// You could do this in other ways: database, or JWT tokens, or both, for example.
	code := s.storeAuthorization(req)

	// Redirect to client callback.
	query := url.Values{}
	query.Set("code", code)
	query.Set("state", req.State)
	http.Redirect(w, r, req.RedirectURI+"?"+query.Encode(), http.StatusFound)
}

// mustAuth is a middleware to ensure that the request is authorized. The way this
// works depends on the implementation. It then stores the scopes in the context.
func (s *server) mustAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer")
		token = strings.TrimSpace(token)

		tk := s.getToken(token)
		if tk == nil {
			serveErrorJSON(w, http.StatusUnauthorized, "invalid_request", "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), scopesContextKey, tk.scopes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func randomString() string {
	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(u)
}
