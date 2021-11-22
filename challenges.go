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

func IsValidCodeChallengeMethod(ccm string) bool {
	return containsString(codeChallengeMethods, ccm)
}

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

func containsString(s []string, v string) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}
