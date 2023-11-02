package indieauth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var validProfileURLs = []string{
	"https://example.com/",
	"https://example.com/username",
	"https://example.com/users?id=100",
}

func TestValidProfileURL(t *testing.T) {
	for _, profileURL := range validProfileURLs {
		err := IsValidProfileURL(profileURL)
		assert.NoError(t, err, profileURL)
	}
}

var invalidProfileURLs = []struct {
	URL   string
	Error error
}{
	{"example.com", ErrInvalidScheme},
	{"mailto:user@example.com", ErrInvalidScheme},
	{"https://example.com/foo/../bar", ErrInvalidPath},
	{"https://example.com/#me", ErrInvalidFragment},
	{"https://user:pass@example.com/", ErrUserIsSet},
	{"https://example.com:8443/", ErrPortIsSet},
	{"https://172.28.92.51/", ErrIsIP},
}

func TestInvalidProfileURL(t *testing.T) {
	for _, test := range invalidProfileURLs {
		err := IsValidProfileURL(test.URL)
		assert.ErrorIs(t, err, test.Error, test.URL)
	}
}

var validClientIdentifiers = []string{
	"https://example.com/",
	"https://example.com/username",
	"https://example.com/users?id=100",
	"https://example.com:8443/",
	"https://127.0.0.1/",
	"https://[::1]/",
}

func TestValidClientIdentifier(t *testing.T) {
	for _, clientID := range validClientIdentifiers {
		err := IsValidClientIdentifier(clientID)
		assert.NoError(t, err, clientID)
	}
}

var invalidClientIdentifier = []struct {
	URL   string
	Error error
}{
	{"example.com", ErrInvalidScheme},
	{"mailto:user@example.com", ErrInvalidScheme},
	{"https://example.com/foo/../bar", ErrInvalidPath},
	{"https://example.com/#me", ErrInvalidFragment},
	{"https://user:pass@example.com/", ErrUserIsSet},
	{"https://172.28.92.51/", ErrIsNonLoopback},
}

func TestInvalidClientIdentifier(t *testing.T) {
	for _, test := range invalidClientIdentifier {
		err := IsValidClientIdentifier(test.URL)
		assert.ErrorIs(t, err, test.Error, test.URL)
	}
}

var canonicalizeTests = [][2]string{
	{"example.com", "https://example.com/"},
	{"http://example.com", "http://example.com/"},
	{"https://example.com", "https://example.com/"},
	{"example.com/", "https://example.com/"},
}

func TestCanonicalizeURL(t *testing.T) {
	for _, test := range canonicalizeTests {
		if CanonicalizeURL(test[0]) != test[1] {
			t.Errorf("canonicalize: expected %s, got %s", test[1], CanonicalizeURL(test[0]))
		}
	}
}

func ExampleCanonicalizeURL() {
	fmt.Println(CanonicalizeURL("example.com"))
	// Output: https://example.com/
}
