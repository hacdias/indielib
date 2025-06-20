package indieauth

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ErrInvalidIssuer error = errors.New("issuer does not match")
)

// Client is an IndieAuth client. As a client, you want to authenticate other users
// to log into your website. An example of how to use the client library can be
// found in the examples/client/ directory.
type Client struct {
	Client *http.Client

	ClientID    string
	RedirectURL string
}

// NewClient creates a new [Client] from the provided clientID and redirectURL.
// If no httpClient is given, [http.DefaultClient] will be used.
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
	Metadata
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

type Metadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	IntrospectionEndpoint                      string   `json:"introspection_endpoint"`
	IntrospectionEndpointAuthMethodsSupported  []string `json:"introspection_endpoint_auth_methods_supported"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	RevocationEndpointAuthMethodsSupported     []string `json:"revocation_endpoint_auth_methods_supported"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	ServiceDocumentation                       []string `json:"service_documentation"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
	UserInfoEndpoint                           string   `json:"userinfo_endpoint"`
}

// Authenticate takes a profile URL and the desired scope, discovers the required
// endpoints, generates a random state and code challenge (using method SHA256),
// and builds the authorization URL. It returns the authorization info, redirect
// URI and an error.
//
// The returned [AuthInfo] should be stored by the caller of this function in
// such a way that it can be retrieved to validate the callback.
func (c *Client) Authenticate(ctx context.Context, profile, scope string) (*AuthInfo, string, error) {
	metadata, err := c.DiscoverMetadata(ctx, profile)
	if err != nil {
		return nil, "", err
	}

	o := &oauth2.Config{
		ClientID:    c.ClientID,
		RedirectURL: c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  metadata.AuthorizationEndpoint,
			TokenURL: metadata.TokenEndpoint,
		},
	}

	state, err := newState()
	if err != nil {
		return nil, "", err
	}
	cv, err := newVerifier()
	if err != nil {
		return nil, "", err
	}

	authURL := o.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("scope", scope),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", s256Challenge(cv)),
	)

	return &AuthInfo{
		Metadata:     *metadata,
		Me:           profile,
		State:        state,
		CodeVerifier: cv,
	}, authURL, nil
}

// newState generates a new state value.
func newState() (string, error) {
	// OAuth 2.0 requires state to be printable ASCII, so base64 fits.
	// See https://datatracker.ietf.org/doc/html/rfc6749#appendix-A.5.
	b := make([]byte, 64)
	_, err := cryptorand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ValidateCallback validates the callback request by checking if the code exists
// and if the state is valid according to the provided [AuthInfo].
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

	// If the issuer is not defined on the metadata, it means that the server does
	// not comply with the newer revision of IndieAuth. In that case, both the metadata
	// issuer and the "iss" should be empty. This should be backwards compatible.
	issuer := r.URL.Query().Get("iss")
	if issuer != i.Issuer {
		return "", ErrInvalidIssuer
	}

	return code, nil
}

// ProfileFromToken retrieves the extra information from the token and creates a
// profile based on it. Note that the profile may be nil in case no information
// can be retrieved.
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

// GetToken exchanges the code for an [oauth2.Token] based on the provided information.
// It returns the token and an [oauth2.Config] object which can be used to create an http
// client that uses the token on future requests.
//
// Note that token.Raw may contain other information returned by the server, such as
// "Me", "Profile" and "Scope".
//
//	token, oauth2, err := client.GetToken(authData, code)
//	if err != nil {
//		// Do something
//	}
//	httpClient := oauth2.Client(context.Background(), token)
//
// You can now use httpClient to make requests to, for example, a Micropub endpoint. They
// are authenticated with token. See https://pkg.go.dev/golang.org/x/oauth2 for more details.
func (c *Client) GetToken(ctx context.Context, i *AuthInfo, code string) (*oauth2.Token, *oauth2.Config, error) {
	if i.TokenEndpoint == "" {
		return nil, nil, ErrNoEndpointFound
	}

	o := c.GetOAuth2(&i.Metadata)

	tok, err := o.Exchange(
		context.WithValue(ctx, oauth2.HTTPClient, c.Client),
		code,
		oauth2.SetAuthURLParam("client_id", c.ClientID),
		oauth2.SetAuthURLParam("code_verifier", i.CodeVerifier),
	)
	if err != nil {
		return nil, nil, err
	}
	return tok, o, nil
}

// GetOAuth2 returns an [oauth2.Config] based on the given endpoints. This can be
// used to get an [http.Client]. See the documentation of [oauth2] for more details.
func (c *Client) GetOAuth2(m *Metadata) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    c.ClientID,
		RedirectURL: c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  m.AuthorizationEndpoint,
			TokenURL: m.TokenEndpoint,
		},
	}
}

// FetchProfile fetches the user [Profile], exchanging the authentication code from
// their authentication endpoint, as per [specification]. Please note that
// this action consumes the code.
//
// [specification]: https://indieauth.spec.indieweb.org/#profile-url-response
func (c *Client) FetchProfile(ctx context.Context, i *AuthInfo, code string) (*Profile, error) {
	v := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURL},
		"client_id":     {c.ClientID},
		"code_verifier": {i.CodeVerifier},
	}

	r, err := http.NewRequestWithContext(ctx, "POST", i.AuthorizationEndpoint, strings.NewReader(v.Encode()))
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
	defer func() {
		_ = res.Body.Close()
	}()

	data, err := io.ReadAll(res.Body)
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
