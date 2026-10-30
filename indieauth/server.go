package indieauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"slices"
	"strings"
)

var (
	ErrInvalidRedirectURI         error = errors.New("redirect_uri is invalid")
	ErrInvalidCodeChallengeMethod error = errors.New("invalid code challenge method")
	ErrInvalidGrantType           error = errors.New("grant_type must be authorization_code")
	ErrNoMatchClientID            error = errors.New("client_id differs")
	ErrNoMatchRedirectURI         error = errors.New("redirect_uri differs")
	ErrPKCERequired               error = errors.New("pkce is required, not provided")
	ErrCodeChallengeFailed        error = errors.New("code challenge failed")
	ErrInvalidResponseType        error = errors.New("response_type must be code")
	ErrWrongCodeVerifierLength    error = errors.New("code_verifier length must be between 43 and 128 characters long")  // RFC 7636, section 4.1
	ErrWrongCodeChallengeLength   error = errors.New("code_challenge length must be between 43 and 128 characters long") // RFC 7636, section 4.2
)

type Server struct {
	Client      *http.Client
	RequirePKCE bool
}

// NewServer creates a new [Server] that from the given options. If
// no httpClient is given, [http.DefaultClient] will be used.
//
// The server fetches client identifiers chosen by whoever sends the request.
// Loopback addresses are never fetched, but domains that resolve to private
// addresses are. To block those, pass an httpClient whose dialer rejects them.
func NewServer(requirePKCE bool, httpClient *http.Client) *Server {
	s := &Server{
		RequirePKCE: requirePKCE,
	}

	if httpClient != nil {
		s.Client = httpClient
	} else {
		s.Client = http.DefaultClient
	}

	return s
}

type AuthenticationRequest struct {
	RedirectURI         string
	ClientID            string
	Scopes              []string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// ParseAuthorization parses an authorization request and returns all the collected
// information about the request.
func (s *Server) ParseAuthorization(r *http.Request) (*AuthenticationRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	resType := r.FormValue("response_type")
	if resType == "" {
		// Default to support legacy clients.
		resType = "code"
	}

	if resType != "code" {
		return nil, ErrInvalidResponseType
	}

	clientID := r.FormValue("client_id")
	if err := IsValidClientIdentifier(clientID); err != nil {
		return nil, err
	}

	redirectURI := r.FormValue("redirect_uri")
	if err := s.validateRedirectURI(r.Context(), clientID, redirectURI); err != nil {
		return nil, err
	}

	var (
		cc  string
		ccm string
	)

	cc = r.Form.Get("code_challenge")
	if cc != "" {
		if len(cc) < 43 || len(cc) > 128 {
			return nil, ErrWrongCodeChallengeLength
		}

		ccm = r.Form.Get("code_challenge_method")
		if !IsValidCodeChallengeMethod(ccm) {
			return nil, ErrInvalidCodeChallengeMethod
		}
	} else if s.RequirePKCE {
		return nil, ErrPKCERequired
	}

	req := &AuthenticationRequest{
		RedirectURI:         redirectURI,
		ClientID:            clientID,
		State:               r.Form.Get("state"),
		Scopes:              []string{},
		CodeChallenge:       cc,
		CodeChallengeMethod: ccm,
	}

	scope := r.Form.Get("scope")
	if scope != "" {
		req.Scopes = strings.Split(scope, " ")
	} else if scopes := r.Form["scopes"]; len(scopes) > 0 {
		req.Scopes = scopes
	}

	return req, nil
}

func (s *Server) validateRedirectURI(ctx context.Context, clientID, redirectURI string) error {
	client, err := urlpkg.Parse(clientID)
	if err != nil {
		return errors.Join(ErrInvalidRedirectURI, err)
	}

	redirect, err := urlpkg.Parse(redirectURI)
	if err != nil {
		return errors.Join(ErrInvalidRedirectURI, err)
	}

	if redirect.Scheme == client.Scheme && redirect.Host == client.Host {
		return nil
	}

	if redirect.Host == "" {
		return errors.Join(ErrInvalidRedirectURI, errors.New("redirect uri has no host"))
	}

	// Redirect URI has a different host, make sure it's published by the client.
	uris, err := s.discoverRedirectURIs(ctx, clientID)
	if err != nil {
		return errors.Join(ErrInvalidRedirectURI, err)
	}

	if slices.Contains(uris, redirectURI) {
		return nil
	}

	return errors.Join(ErrInvalidRedirectURI, errors.New("redirect uri not published by client"))
}

// discoverRedirectURIs fetches the redirect URIs either published in the client
// metadata, or in the <link> relations if the response is HTML.
// - https://indieauth.spec.indieweb.org/#client-metadata
// - https://indieauth.spec.indieweb.org/#redirect-url
func (s *Server) discoverRedirectURIs(ctx context.Context, clientID string) ([]string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, err
	}
	if isLoopback(r.URL) {
		return nil, ErrLoopbackClientID
	}
	r.Header.Add("Accept", "application/json, text/html")

	res, err := s.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: expected 200, got %d", res.StatusCode)
	}

	if strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		return extractAllLinks(res, RedirectURIRel)
	}

	var metadata struct {
		RedirectURIs []string `json:"redirect_uris"`
	}

	if err := json.NewDecoder(res.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	return metadata.RedirectURIs, nil
}

// ValidateTokenExchange validates the token exchange request according to the
// provided authentication request and returns an error.
//
// Please note that you need to fetch the authentication code yourself from the
// request.
//
//	_ = r.ParseForm()
//	code := r.Form.Get("code")
//
// The code was provided by you at a previous stage. Thus, you will need to use
// it to rebuild the [AuthenticationRequest] data. The [AuthenticationRequest]
// does not need to have the scope or state set for this validation.
func (s *Server) ValidateTokenExchange(authRequest *AuthenticationRequest, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	grantType := r.Form.Get("grant_type")
	if grantType == "" {
		// Default to support legacy clients.
		grantType = "authorization_code"
	}

	if grantType != "authorization_code" {
		return ErrInvalidGrantType
	}

	var (
		clientID    = r.Form.Get("client_id")
		redirectURI = r.Form.Get("redirect_uri")
	)

	if authRequest.ClientID != clientID {
		return ErrNoMatchClientID
	}

	if authRequest.RedirectURI != redirectURI {
		return ErrNoMatchRedirectURI
	}

	if authRequest.CodeChallenge == "" {
		if s.RequirePKCE {
			return ErrPKCERequired
		}
	} else {
		codeVerifier := r.Form.Get("code_verifier")
		if len(codeVerifier) < 43 || len(codeVerifier) > 128 {
			return ErrWrongCodeVerifierLength
		}
		cc := authRequest.CodeChallenge
		if len(cc) < 43 || len(cc) > 128 {
			return ErrWrongCodeChallengeLength
		}
		ccm := authRequest.CodeChallengeMethod
		if !IsValidCodeChallengeMethod(ccm) {
			return ErrInvalidCodeChallengeMethod
		}

		if !ValidateCodeChallenge(ccm, cc, codeVerifier) {
			return ErrCodeChallengeFailed
		}
	}

	return nil
}
