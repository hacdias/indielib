package indieauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"willnorris.com/go/microformats"
	"willnorris.com/go/webmention/third_party/header"
)

const (
	AuthorizationEndpointRel string = "authorization_endpoint"
	TokenEndpointRel         string = "token_endpoint"
	IndieAuthMetadataRel     string = "indieauth-metadata"
)

// ErrNoEndpointFound is returned when no endpoint can be found for a certain
// target URL.
var ErrNoEndpointFound = fmt.Errorf("no endpoint found")

// DiscoverMetadata discovers the IndieAuth metadata of the provided URL, such
// as the authorization and token endpoints. This code is partially based on
// [webmention.DiscoverEndpoint].
//
// [webmention.DiscoverEndpoint]: https://github.com/willnorris/webmention/blob/main/webmention.go
func (c *Client) DiscoverMetadata(ctx context.Context, urlStr string) (*Metadata, error) {
	metadata, err := c.discoverMetadata(ctx, urlStr)
	if err == nil {
		return metadata, nil
	}

	// This part is kept as means of backwards compatibility with IndieAuth revision from
	// 26 November 2020: https://indieauth.spec.indieweb.org/20201126/#discovery-by-clients
	urls, err := c.discoverEndpoints(ctx, urlStr, AuthorizationEndpointRel, TokenEndpointRel)
	if err != nil {
		return nil, err
	}

	endpoints := &Metadata{
		AuthorizationEndpoint: urls[0].value,
		TokenEndpoint:         urls[1].value,
		RevocationEndpoint:    urls[1].value,
	}

	// Authorization is mandatory!
	if urls[0].err != nil {
		return nil, urls[0].err
	}

	return endpoints, nil
}

// discoverMetadata fetches the server's metadata information as described in the
// specification: https://indieauth.spec.indieweb.org/#discovery-by-clients
func (c *Client) discoverMetadata(ctx context.Context, urlStr string) (*Metadata, error) {
	metadataUrl, err := c.DiscoverLinkEndpoint(ctx, urlStr, IndieAuthMetadataRel)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataUrl, nil)
	if err != nil {
		return nil, err
	}
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

	var metadata *Metadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// DiscoverLinkEndpoint discovers as given endpoint identified by rel.
func (c *Client) DiscoverLinkEndpoint(ctx context.Context, urlStr, rel string) (string, error) {
	urls, err := c.discoverEndpoints(ctx, urlStr, rel)
	if err != nil {
		return "", err
	}

	return urls[0].value, urls[0].err
}

type endpointRequest struct {
	value string
	err   error
}

func (c *Client) discoverEndpoints(ctx context.Context, urlStr string, rels ...string) ([]*endpointRequest, error) {
	headEndpoints, found, errHead := c.discoverRequest(ctx, http.MethodHead, urlStr, rels...)
	if errHead == nil && headEndpoints != nil && found {
		return headEndpoints, nil
	}

	getEndpoints, found, errGet := c.discoverRequest(ctx, http.MethodGet, urlStr, rels...)
	if errGet == nil && getEndpoints != nil && found {
		return getEndpoints, nil
	}

	if errHead != nil && errGet != nil {
		return nil, errGet
	}

	endpoints := make([]*endpointRequest, len(rels))
	for i := range endpoints {
		if errHead == nil && headEndpoints[i].err == nil {
			endpoints[i] = headEndpoints[i]
		} else if errGet == nil && getEndpoints[i].err == nil {
			endpoints[i] = getEndpoints[i]
		} else {
			endpoints[i] = &endpointRequest{err: ErrNoEndpointFound}
		}
	}
	return endpoints, nil
}

func (c *Client) discoverRequest(ctx context.Context, method, urlStr string, rels ...string) ([]*endpointRequest, bool, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if code := resp.StatusCode; code < 200 || 300 <= code {
		return nil, false, fmt.Errorf("response error: %v", resp.StatusCode)
	}

	endpoints, found, err := extractEndpoints(resp, rels...)
	if err != nil {
		return nil, false, err
	}

	err = resolveReferences(resp.Request.URL.String(), endpoints...)
	if err != nil {
		return nil, false, err
	}

	return endpoints, found, nil
}

func extractEndpoints(resp *http.Response, rels ...string) ([]*endpointRequest, bool, error) {
	// first check http link headers
	httpEndpoints, found := httpLink(resp.Header, rels...)
	if found {
		return httpEndpoints, true, nil
	}

	// then look in the HTML body
	htmlEndpoints, _, err := htmlLink(resp.Body, rels...)
	if err != nil {
		return nil, false, err
	}

	endpoints := make([]*endpointRequest, len(rels))
	matched := 0
	for i := range endpoints {
		if httpEndpoints[i].err == nil {
			endpoints[i] = httpEndpoints[i]
		} else {
			endpoints[i] = htmlEndpoints[i]
		}
		if endpoints[i].err == nil {
			matched++
		}
	}
	return endpoints, matched == len(rels), nil
}

// httpLink parses headers and returns the URL of the first link that contains a rel value.
func httpLink(headers http.Header, rels ...string) ([]*endpointRequest, bool) {
	links := make([]*endpointRequest, len(rels))
	matched := 0

	for _, h := range header.ParseList(headers, "Link") {
		link := header.ParseLink(h)
		for _, v := range link.Rel {
			for i, rel := range rels {
				if v == rel && links[i] == nil {
					links[i] = &endpointRequest{value: link.Href}
					matched++
				}
			}
		}
	}

	for i := range links {
		if links[i] == nil {
			links[i] = &endpointRequest{err: ErrNoEndpointFound}
		}
	}

	return links, matched == len(links)
}

// htmlLink parses r as HTML and returns the URLs of the first link that
// contains the rels values. HTML <link> elements are preferred, falling back
// to <a> elements if no rel <link> elements are found.
func htmlLink(r io.Reader, rels ...string) ([]*endpointRequest, bool, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, false, err
	}

	var f func(n *html.Node, targetRel string) *endpointRequest
	f = func(n *html.Node, targetRel string) *endpointRequest {
		if n.Type == html.ElementNode {
			if n.DataAtom == atom.Link || n.DataAtom == atom.A {
				var href, rel string
				var hrefFound, relFound bool
				for _, a := range n.Attr {
					if a.Key == atom.Href.String() {
						href = a.Val
						hrefFound = true
					}
					if a.Key == atom.Rel.String() {
						rel = a.Val
						relFound = true
					}
				}
				if hrefFound && relFound {
					for _, v := range strings.Split(rel, " ") {
						if v == targetRel {
							return &endpointRequest{value: href}
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if link := f(c, targetRel); link.err == nil {
				return link
			}
		}
		return &endpointRequest{err: ErrNoEndpointFound}
	}

	links := make([]*endpointRequest, len(rels))
	matched := 0
	for i, rel := range rels {
		links[i] = f(doc, rel)
		if links[i].err == nil {
			matched++
		}
	}

	return links, matched == len(rels), nil
}

// resolveReferences resolves each URL in refs into an absolute URL relative to
// base. If base or one of the values in refs is not a valid URL, an error is returned.
func resolveReferences(base string, refs ...*endpointRequest) error {
	b, err := url.Parse(base)
	if err != nil {
		return err
	}

	for _, r := range refs {
		if r.err == nil {
			u, err := url.Parse(r.value)
			if err != nil {
				return err
			}
			r.value = b.ResolveReference(u).String()
		}
	}
	return nil
}

type ApplicationMetadata struct {
	Name    string
	Logo    string
	URL     string
	Summary string
	Author  string
}

// ErrNoApplicationMetadata is returned when no `h-app` or `h-x-app` Microformat
// has been found at a given URL.
var ErrNoApplicationMetadata error = errors.New("application metadata (h-app, h-x-app) not found")

// DiscoverApplicationMetadata fetches metadata for the application at the
// provided URL. This metadata is given by the `h-app` or `h-x-app` [Microformat].
// This information can be used by the server, for example, to display relevant
// information about the application in the authorization page. If no information
// has been found, an [ErrNoApplicationMetadata] error will be returned.
//
// Please note that this function only parses the first `h-app` or `h-x-app`
// Microformat with information that it encounters.
//
// [Microformat]: https://microformats.org/wiki/h-app
func (s *Server) DiscoverApplicationMetadata(ctx context.Context, clientID string) (*ApplicationMetadata, error) {
	err := IsValidClientIdentifier(clientID)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Add("Accept", "text/html")

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

	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return nil, fmt.Errorf("content-type: expected to include text/html, got '%q'", contentType)
	}

	data := microformats.Parse(res.Body, res.Request.URL)
	if data == nil {
		return nil, ErrNoApplicationMetadata
	}

	for _, item := range data.Items {
		isApp := false
		for _, typ := range item.Type {
			// h-x-app for legacy support
			if typ == "h-app" || typ == "h-x-app" {
				isApp = true
				break
			}
		}
		if !isApp {
			continue
		}

		name := getFirstStringProperty(item, "name")
		url := getFirstStringProperty(item, "url")
		logo := getFirstStringProperty(item, "logo")
		if logo == "" {
			logo = getFirstStringProperty(item, "photo")
		}
		summary := getFirstStringProperty(item, "summary")
		author := getFirstStringProperty(item, "author")

		if name == "" && url == "" && logo == "" && summary == "" && author == "" {
			continue
		}

		return &ApplicationMetadata{
			Name:    name,
			URL:     url,
			Logo:    logo,
			Summary: summary,
			Author:  author,
		}, nil
	}

	return nil, ErrNoApplicationMetadata
}

func getFirstStringProperty(item *microformats.Microformat, key string) string {
	vv, ok := item.Properties[key]
	if !ok {
		return ""
	}

	for _, v := range vv {
		if s, ok := v.(string); ok && s != "" {
			return s
		}

		if m, ok := v.(map[string]string); ok {
			if mv, ok := m["value"]; ok && mv != "" {
				return mv
			}
		}
	}

	return ""
}
