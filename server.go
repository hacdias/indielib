package indieauth

import (
	"errors"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"strings"
)

type Server struct {
	Client *http.Client

	RequirePKCE bool
}

type AuthenticationRequest struct {
	RedirectURI         string
	ClientID            string
	Scopes              []string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

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
		return nil, errors.New("response_type must be code")
	}

	clientID := r.FormValue("client_id")
	if err := IsValidClientIdentifier(clientID); err != nil {
		return nil, fmt.Errorf("invalid client_id: %w", err)
	}

	redirectURI := r.FormValue("redirect_uri")
	if err := s.validateRedirectURI(clientID, redirectURI); err != nil {
		return nil, err
	}

	var (
		cc  string
		ccm string
	)

	cc = r.Form.Get("code_challenge")
	if cc != "" {
		if len(cc) < 43 || len(cc) > 128 {
			return nil, errors.New("code_challenge length must be between 43 and 128 charachters long")
		}

		ccm = r.Form.Get("code_challenge_method")
		if !IsValidCodeChallengeMethod(ccm) {
			return nil, errors.New("code_challenge_method not supported")
		}
	} else if s.RequirePKCE {
		return nil, errors.New("code_challenge and code_challenge_method are required")
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

func (s *Server) validateRedirectURI(clientID, redirectURI string) error {
	client, err := urlpkg.Parse(clientID)
	if err != nil {
		return err
	}

	redirect, err := urlpkg.Parse(redirectURI)
	if err != nil {
		return err
	}

	if redirect.Host == client.Host {
		return nil
	}

	// TODO: redirect URI may have a different host. In this case, we do
	// discovery: https://indieauth.spec.indieweb.org/#redirect-url
	return errors.New("redirect uri has different host from client id")
}

func (s *Server) ParseCodeExchangeRequest(r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	return nil
}
