package indieauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
)

type Client struct {
	Client *http.Client

	ClientID    string
	RedirectURL string
}

type AuthInfo struct {
	Endpoints
	Me           string
	State        string
	CodeVerifier string
}

type Profile struct {
	Me      string `json:"me"`
	Profile struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Photo string `json:"photo"`
		Email string `json:"email"`
	} `json:"profile"`
}

// Authenticate takes a profile URL and the desired scope, discovers the required endpoints,
// generates a random scope and code challenge (using method SHA256), and builds the authorization
// URL. It returns the authorization info, redirect URI and an error.
//
// The returned AuthInfo should be stored by the caller of this function in such a way that it
// can be retrieved to validate the callback.
func (c *Client) Authenticate(profile, scope string) (*AuthInfo, string, error) {
	endpoints, err := c.DiscoverEndpoints(profile)
	if err != nil {
		return nil, "", err
	}

	o := &oauth2.Config{
		ClientID:    c.ClientID,
		RedirectURL: c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoints.Authorization,
			TokenURL: endpoints.Token,
		},
	}

	state := randString(20)
	codeVerifier := randString(20)

	sha256 := sha256.Sum256([]byte(codeVerifier))
	cc := base64.URLEncoding.EncodeToString(sha256[:])

	authURL := o.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("scope", scope),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", cc),
	)

	return &AuthInfo{
		Endpoints:    *endpoints,
		Me:           profile,
		State:        state,
		CodeVerifier: codeVerifier,
	}, authURL, nil
}

// ValidateCallback validates the callback request by checking if the code exists
// and if the state is valid according to the provided AuthInfo.
func (c *Client) ValidateCallback(i *AuthInfo, r *http.Request) (string, error) {
	code := r.URL.Query().Get("code")
	if code == "" {
		return "", errors.New("code query not found")
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		return "", errors.New("state query not found")
	}

	if state != i.State {
		return "", errors.New("state does not match")
	}

	return code, nil
}

// ProfileFromToken retrieves the extra information from the token and
// creates a profile based on it. Note that the profile may be nil in case
// no information can be retrieved.
func ProfileFromToken(token *oauth2.Token) *Profile {
	me, ok := token.Extra("me").(string)
	if !ok || me == "" {
		return nil
	}

	p := &Profile{
		Me: me,
	}

	profile, ok := token.Extra("profile").(map[string]interface{})
	if !ok {
		return p
	}

	if name, ok := profile["name"].(string); ok {
		p.Profile.Name = name
	}

	if url, ok := profile["url"].(string); ok {
		p.Profile.URL = url
	}

	if photo, ok := profile["photo"].(string); ok {
		p.Profile.Photo = photo
	}

	if email, ok := profile["email"].(string); ok {
		p.Profile.Email = email
	}

	return p
}

// GetToken exchanges the code for an oauth2.Token based on the provided information.
// It returns the token and an oauth2.Config object which can be used to create an http
// client that uses the token on future requests.
//
// Note that token.Raw may contain other information returned by the server, such as
// "Me", "Profile" and "Scope".
func (c *Client) GetToken(i *AuthInfo, code string) (*oauth2.Token, *oauth2.Config, error) {
	o := &oauth2.Config{
		ClientID:    c.ClientID,
		RedirectURL: c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  i.Authorization,
			TokenURL: i.Token,
		},
	}

	tok, err := o.Exchange(
		context.Background(),
		code,
		oauth2.SetAuthURLParam("client_id", c.ClientID),
		oauth2.SetAuthURLParam("code_verifier", i.CodeVerifier),
	)
	if err != nil {
		return nil, nil, err
	}

	return tok, o, nil
}

// FetchProfile fetches the user profile, exchanging the authentication code from
// their authentication endpoint, as described in the link below. Please note that
// this action consumes the code.
//
// https://indieauth.spec.indieweb.org/#profile-url-response
func (c *Client) FetchProfile(i *AuthInfo, code string) (*Profile, error) {
	v := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURL},
		"client_id":     {c.ClientID},
		"code_verifier": {i.CodeVerifier},
	}

	r, err := http.NewRequest("POST", i.Authorization, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(v.Encode())))
	r.Header.Add("Accept", "application/json")

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: expected 200, got %d", res.StatusCode)
	}

	var profile *Profile
	err = json.Unmarshal(data, &profile)
	if err != nil {
		return nil, err
	}

	return profile, nil
}
