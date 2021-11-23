package indieauth

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"willnorris.com/go/webmention/third_party/header"
)

const (
	AuthorizationEndpointRel string = "authorization_endpoint"
	TokenEndpointRel         string = "token_endpoint"
)

type Endpoints struct {
	Authorization string
	Token         string
}

// ErrNoEndpointFound is returned when no endpoint can be found for a certain
// target URL.
var ErrNoEndpointFound = fmt.Errorf("no endpoint found")

// DiscoverEndpoints discovers the authorization and token endpoints for the provided URL.
// This code is partially based on https://github.com/willnorris/webmention/blob/main/webmention.go.
func (s *Client) DiscoverEndpoints(urlStr string) (*Endpoints, error) {
	urls, err := s.discoverEndpoints(urlStr, AuthorizationEndpointRel, TokenEndpointRel)
	if err != nil {
		return nil, err
	}

	return &Endpoints{
		Authorization: urls[0],
		Token:         urls[1],
	}, nil
}

// DiscoverEndpoint discovers as given endpoint identified by rel.
func (s *Client) DiscoverEndpoint(urlStr, rel string) (string, error) {
	urls, err := s.discoverEndpoints(urlStr, rel)
	if err != nil {
		return "", err
	}

	return urls[0], nil
}

func (s *Client) discoverEndpoints(urlStr string, rels ...string) ([]string, error) {
	headEndpoints, err := s.discoverRequest(http.MethodHead, urlStr, rels...)
	if err == nil && headEndpoints != nil {
		return headEndpoints, nil
	}

	getEndpoints, err := s.discoverRequest(http.MethodGet, urlStr, rels...)
	if err == nil && getEndpoints != nil {
		return getEndpoints, nil
	}

	return nil, err
}

func (s *Client) discoverRequest(method, urlStr string, rels ...string) ([]string, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code < 200 || 300 <= code {
		return nil, fmt.Errorf("response error: %v", resp.StatusCode)
	}

	endpoints, err := extractEndpoints(resp, rels...)
	if err != nil {
		return nil, err
	}

	urls, err := resolveReferences(resp.Request.URL.String(), endpoints...)
	if err != nil {
		return nil, err
	}

	return urls, nil
}

func extractEndpoints(resp *http.Response, rels ...string) ([]string, error) {
	// first check http link headers
	if endpoints, err := httpLink(resp.Header, rels...); err == nil {
		return endpoints, nil
	}

	// then look in the HTML body
	endpoints, err := htmlLink(resp.Body, rels...)
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

// httpLink parses headers and returns the URL of the first link that contains a rel value.
func httpLink(headers http.Header, rels ...string) ([]string, error) {
	links := make([]string, len(rels))
	found := make([]bool, len(rels))
	matched := 0

	for _, h := range header.ParseList(headers, "Link") {
		link := header.ParseLink(h)
		for _, v := range link.Rel {
			for i, rel := range rels {
				if v == rel && !found[i] {
					links[i] = link.Href
					found[i] = true
					matched++

					if matched == len(rels) {
						return links, nil
					}
				}
			}
		}
	}

	return nil, ErrNoEndpointFound
}

// htmlLink parses r as HTML and returns the URLs of the first link that
// contains the rels values. HTML <link> elements are preferred, falling back
// to <a> elements if no rel <link> elements are found.
func htmlLink(r io.Reader, rels ...string) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var f func(n *html.Node, targetRel string) (string, error)
	f = func(n *html.Node, targetRel string) (string, error) {
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
							return href, nil
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if link, err := f(c, targetRel); err == nil {
				return link, nil
			}
		}
		return "", ErrNoEndpointFound
	}

	links := make([]string, len(rels))
	for i, rel := range rels {
		links[i], err = f(doc, rel)
		if err != nil {
			return nil, err
		}
	}

	return links, nil
}

// resolveReferences resolves each URL in refs into an absolute URL relative to
// base. If base or one of the values in refs is not a valid URL, an error is returned.
func resolveReferences(base string, refs ...string) ([]string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}

	var urls []string
	for _, r := range refs {
		u, err := url.Parse(r)
		if err != nil {
			return nil, err
		}
		urls = append(urls, b.ResolveReference(u).String())
	}
	return urls, nil
}
