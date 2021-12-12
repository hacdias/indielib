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

	endpoints := &Endpoints{
		Authorization: urls[0].value,
		Token:         urls[1].value,
	}

	// Authorization is mandatory!
	if urls[0].err != nil {
		return nil, urls[0].err
	}

	return endpoints, nil
}

// DiscoverEndpoint discovers as given endpoint identified by rel.
func (s *Client) DiscoverEndpoint(urlStr, rel string) (string, error) {
	urls, err := s.discoverEndpoints(urlStr, rel)
	if err != nil {
		return "", err
	}

	return urls[0].value, urls[0].err
}

type endpointRequest struct {
	value string
	err   error
}

func (s *Client) discoverEndpoints(urlStr string, rels ...string) ([]*endpointRequest, error) {
	headEndpoints, found, errHead := s.discoverRequest(http.MethodHead, urlStr, rels...)
	if errHead == nil && headEndpoints != nil && found {
		return headEndpoints, nil
	}

	getEndpoints, found, errGet := s.discoverRequest(http.MethodGet, urlStr, rels...)
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

func (s *Client) discoverRequest(method, urlStr string, rels ...string) ([]*endpointRequest, bool, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

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
