package indieauth

import (
	"errors"
	"net/netip"
	urlpkg "net/url"
	"strings"
)

var (
	ErrInvalidProfileURL       error = errors.New("invalid profile URL")
	ErrInvalidClientIdentifier error = errors.New("invalid client identifier")

	ErrInvalidScheme   error = errors.New("scheme must be either http or https")
	ErrEmptyPath       error = errors.New("path must not be empty")
	ErrInvalidPath     error = errors.New("path cannot contain single-dot or double-dot segments")
	ErrInvalidFragment error = errors.New("fragment must be empty")
	ErrUserIsSet       error = errors.New("user and or password must not be set")
	ErrPortIsSet       error = errors.New("port must not be set")
	ErrIsIP            error = errors.New("profile cannot be ip address")
	ErrIsNonLoopback   error = errors.New("client id cannot be non-loopback ip")
)

// IsValidProfileURL validates the profile URL according to the [specification].
//
// [specification]: https://indieauth.spec.indieweb.org/#user-profile-url
func IsValidProfileURL(profile string) error {
	url, err := parseURL(profile)
	if err != nil {
		return errors.Join(ErrInvalidProfileURL, err)
	}

	if url.Port() != "" {
		return errors.Join(ErrInvalidProfileURL, ErrPortIsSet)
	}

	if _, err := netip.ParseAddr(url.Hostname()); err == nil {
		return errors.Join(ErrInvalidProfileURL, ErrIsIP)
	}

	return nil
}

// IsValidClientIdentifier validates a client identifier according to the [specification].
//
// [specification]: https://indieauth.spec.indieweb.org/#client-identifier
func IsValidClientIdentifier(identifier string) error {
	url, err := parseURL(identifier)
	if err != nil {
		return errors.Join(ErrInvalidClientIdentifier, err)
	}

	if v, err := netip.ParseAddr(url.Hostname()); err == nil {
		if !v.IsLoopback() {
			return errors.Join(ErrInvalidClientIdentifier, ErrIsNonLoopback)
		}
	}

	return nil
}

// parseURL parses a URL and checks the rules shared by profile URLs and client
// identifiers.
func parseURL(rawURL string) (*urlpkg.URL, error) {
	url, err := urlpkg.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return nil, ErrInvalidScheme
	}

	if url.Path == "" {
		return nil, ErrEmptyPath
	}

	for _, segment := range strings.Split(url.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, ErrInvalidPath
		}
	}

	if url.Fragment != "" {
		return nil, ErrInvalidFragment
	}

	if url.User.String() != "" {
		return nil, ErrUserIsSet
	}

	return url, nil
}

// CanonicalizeURL adds an https scheme if the URL has none, lowercases the
// host, and appends a path "/" if it has no path.
func CanonicalizeURL(urlStr string) string {
	// NOTE: parsing a URL without scheme will most likely put the host as path.
	// That's why I add it first.
	if s := strings.ToLower(urlStr); !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		urlStr = "https://" + urlStr
	}

	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	url.Host = strings.ToLower(url.Host)
	if url.Path == "" {
		url.Path = "/"
	}

	return url.String()
}
