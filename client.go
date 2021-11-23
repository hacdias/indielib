package indieauth

import (
	"context"
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

var (
	ErrCodeNotFound  error = errors.New("code not found")
	ErrStateNotFound error = errors.New("state not found")
	ErrInvalidState  error = errors.New("state does not match")
)

// Client is a IndieAuth client. As a client, you want to authenticate other users
// to log into onto your website.
//
// First, create a client with the correct client ID and callback URL.
//
//	client := NewClient("https://example.com/", "https://example.com/callback", nil)
//
// Then, obtain the user's profile URL via some method, such as an HTTP form. Optionally,
// canonicalize the value (see CanonicalizeURL for more information).
//
//  profile = CanonicalizeURL(profile)
//
// Then, validate the profile URL according to the specification.
//
// 	err = IsValidProfileURL(profile)
// 	if err != nil {
// 		// Do something
// 	}
//
// Obtain the authentication information and redirect URL:
//
// 	authData, redirect, err := client.Authenticate(profile, "the scopes you need")
// 	if err != nil {
// 		// Do something
// 	}
//
// The client should now store authData because it will be necessary to verify the callback.
// You can store it, for example, in a database or cookie. Then, redirect the user:
//
// 	http.Redirect(w, r, redirect, http.StatusSeeOther)
//
// In the callback handler, you should obtain authData according to the method you defined.
// Then, call ValidateCallback to obtain the code:
//
//	code, err := client.ValidateCallback(authData, r)
// 	if err != nil {
//		// Do something
// 	}
//
// Now that you have the code, you have to redeem it. You can either use FetchProfile to
// redeem it by the users' profile or GetToken.
type Client struct {
	Client *http.Client

	ClientID    string
	RedirectURL string
}

// NewClient creates a new Client from the provided clientID and redirectURL. If
// no httpClient is given, http.DefaultClient will be used.
func NewClient(clientID, redirectURL string, httpClient *http.Client) *Client {
	c := &Client{
		ClientID:    clientID,
		RedirectURL: redirectURL,
	}

	if httpClient != nil {
		c.Client = httpClient
	} else {
		c.Client = http.DefaultClient
	}

	return c
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
	cc, cv := generateS256Challenge()

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
		CodeVerifier: cv,
	}, authURL, nil
}

// ValidateCallback validates the callback request by checking if the code exists
// and if the state is valid according to the provided AuthInfo.
func (c *Client) ValidateCallback(i *AuthInfo, r *http.Request) (string, error) {
	code := r.URL.Query().Get("code")
	if code == "" {
		return "", ErrCodeNotFound
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		return "", ErrStateNotFound
	}

	if state != i.State {
		return "", ErrInvalidState
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
//
//	token, oauth2, err := client.GetToken(authData, code)
//	if err != nil {
//		// Do something
// 	}
//	httpClient := oauth2.Client(context.Background(), token)
//
// You can now use httpClient to make requests to, for example, a Micropub endpoint. They
// are authenticated with token. See https://pkg.go.dev/golang.org/x/oauth2 for more details.
func (c *Client) GetToken(i *AuthInfo, code string) (*oauth2.Token, *oauth2.Config, error) {
	o := c.GetOAuth2(&i.Endpoints)

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

// GetOAuth2 returns an oauth2.Config based on the given endpoints. This can be used
// to get an http.Client See https://pkg.go.dev/golang.org/x/oauth2 for more details.
func (c *Client) GetOAuth2(e *Endpoints) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    c.ClientID,
		RedirectURL: c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  e.Authorization,
			TokenURL: e.Token,
		},
	}
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
