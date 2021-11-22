package indieauth

import (
	"errors"
	"net"
	urlpkg "net/url"
	"strings"
)

// IsValidProfileURL validates the profile URL according to the specification.
// https://indieauth.spec.indieweb.org/#user-profile-url
func IsValidProfileURL(profile string) error {
	url, err := urlpkg.Parse(profile)
	if err != nil {
		return err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return errors.New("scheme must be either http or https")
	}

	if url.Path == "" {
		return errors.New("path must not be empty")
	}

	if strings.Contains(url.Path, ".") || strings.Contains(url.Path, "..") {
		return errors.New("cannot contain single or double dots")
	}

	if url.Fragment != "" {
		return errors.New("fragment must be empty")
	}

	if url.User.String() != "" {
		return errors.New("user and or password must not be set")
	}

	if url.Port() != "" {
		return errors.New("port must not be set")
	}

	if net.ParseIP(profile) != nil {
		return errors.New("profile cannot be ip address")
	}

	return nil
}

// IsValidClientIdentifier validates a client identifier according to the specification.
// https://indieauth.spec.indieweb.org/#client-identifier
func IsValidClientIdentifier(identifier string) error {
	url, err := urlpkg.Parse(identifier)
	if err != nil {
		return err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return errors.New("scheme must be either http or https")
	}

	if url.Path == "" {
		return errors.New("path must not be empty")
	}

	if strings.Contains(url.Path, ".") || strings.Contains(url.Path, "..") {
		return errors.New("cannot contain single or double dots")
	}

	if url.Fragment != "" {
		return errors.New("fragment must be empty")
	}

	if url.User.String() != "" {
		return errors.New("user and or password must not be set")
	}

	if v := net.ParseIP(identifier); v != nil {
		if !v.IsLoopback() {
			return errors.New("client id cannot be non-loopback ip")
		}
	}

	return nil
}

// CanonicalizeURL checks if a URL has a path, and appends a path "/""
// if it has no path.
func CanonicalizeURL(urlStr string) string {
	// NOTE: parsing a URL without scheme will most likely put the host as path.
	// That's why I add it first.
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	if url.Path == "" {
		url.Path = "/"
	}

	return url.String()
}
