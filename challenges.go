package indieauth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

var (
	codeChallengeMethods = []string{
		"plain", "S256",
	}
)

// IsValidCodeChallengeMethod returns whether the provided code challenge method
// is or is not valid.
func IsValidCodeChallengeMethod(ccm string) bool {
	return containsString(codeChallengeMethods, ccm)
}

// ValidateCodeChallenge validates a code challenge against it code verifier. Right now,
// we support "plain" and "S256" code challenge methods.
func ValidateCodeChallenge(ccm, cc, ver string) bool {
	switch ccm {
	case "plain":
		return cc == ver
	case "S256":
		s256 := sha256.Sum256([]byte(ver))
		// trim padding
		a := strings.TrimRight(base64.URLEncoding.EncodeToString(s256[:]), "=")
		b := strings.TrimRight(cc, "=")
		return a == b
	default:
		return false
	}
}

func generateS256Challenge() (cc string, cv string) {
	cv = randString(20)
	sha256 := sha256.Sum256([]byte(cv))
	cc = base64.URLEncoding.EncodeToString(sha256[:])
	return cc, cv
}

func containsString(s []string, v string) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}
