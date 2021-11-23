package indieauth

import (
	"errors"
	"net"
	urlpkg "net/url"
	"strings"
)

var (
	ErrInvalidScheme   error = errors.New("scheme must be either http or https")
	ErrEmptyPath       error = errors.New("path must not be empty")
	ErrInvalidPath     error = errors.New("path cannot contain single or double dots")
	ErrInvalidFragment error = errors.New("fragment must be empty")
	ErrUserIsSet       error = errors.New("user and or password must not be set")
	ErrPortIsSet       error = errors.New("port must not be set")
	ErrIsIP            error = errors.New("profile cannot be ip address")
	ErrIsNonLoopback   error = errors.New("client id cannot be non-loopback ip")
)

// IsValidProfileURL validates the profile URL according to the specification.
// https://indieauth.spec.indieweb.org/#user-profile-url
func IsValidProfileURL(profile string) error {
	url, err := urlpkg.Parse(profile)
	if err != nil {
		return err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return ErrInvalidScheme
	}

	if url.Path == "" {
		return ErrEmptyPath
	}

	if strings.Contains(url.Path, ".") || strings.Contains(url.Path, "..") {
		return ErrInvalidPath
	}

	if url.Fragment != "" {
		return ErrInvalidFragment
	}

	if url.User.String() != "" {
		return ErrUserIsSet
	}

	if url.Port() != "" {
		return ErrPortIsSet
	}

	if net.ParseIP(url.Host) != nil {
		return ErrIsIP
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
		return ErrInvalidScheme
	}

	if url.Path == "" {
		return ErrEmptyPath
	}

	if strings.Contains(url.Path, ".") || strings.Contains(url.Path, "..") {
		return ErrInvalidPath
	}

	if url.Fragment != "" {
		return ErrInvalidFragment
	}

	if url.User.String() != "" {
		return ErrUserIsSet
	}

	if v := net.ParseIP(url.Host); v != nil {
		if !v.IsLoopback() {
			return ErrIsNonLoopback
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
