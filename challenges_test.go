package indieauth

import "testing"

func TestPlainChallenge(t *testing.T) {
	if !ValidateCodeChallenge("plain", "string", "string") {
		t.Error("plain challenge must be valid")
	}
}

func TestS256Challenge(t *testing.T) {
	cc, cv := generateS256Challenge()
	if !ValidateCodeChallenge("S256", cc, cv) {
		t.Error("S256 challenge must be valid")
	}
}
