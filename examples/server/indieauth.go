package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.hacdias.com/indiekit/indieauth"
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

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// authorizationGetHandler handles the GET method for the authorization endpoint.
func (s *server) authorizationGetHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the authorization request.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In a production server, this page would usually be protected. In order for
	// the user to authorize this request, they must be authenticated. This could
	// be done in different ways: username/password, passkeys, etc.

	// Here, we just display a small HTML document where the user has to press
	// to authorize this request. Please note that this template contains a form
	// where we dump all the request information. This makes it possible to reuse
	// [indieauth.Server.ParseAuthorization] when the user authorizes the request.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.ExecuteTemplate(w, "auth.html", req)
}

// authorizationPostHandler handles the POST method for the authorization endpoint.
func (s *server) authorizationPostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t := s.getAuthorization(r.Form.Get("code"))
	if t == nil {
		http.Error(w, "invalid authorization", http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile := &indieauth.Profile{
		Me: s.profileURL,
	}
	_ = json.NewEncoder(w).Encode(profile)
}

func (s *server) authorizationAcceptHandler(w http.ResponseWriter, r *http.Request) {
	// Parse authorization information. This only works because our authorization page
	// includes all the required information. This can be done in other ways: database,
	// whether temporary or not, cookies, etc.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

// storeAuthorization stores the authorization request and returns a code for it.
func (s *server) storeAuthorization(req *indieauth.AuthenticationRequest) string {
	s.authorizationsMu.Lock()
	defer s.authorizationsMu.Unlock()

	code := randomCode()
	s.authorizations[code] = &authorization{
		req:  req,
		time: time.Now(),
	}

	return code
}

func randomCode() string {
	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(u)
}
