package indieauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestPlainChallenge(t *testing.T) {
	// Generate a code_verifier within the allowed minimum length of 43 characters and maximum
	// length of 128 characters, per https://datatracker.ietf.org/doc/html/rfc7636#section-4.1.
	if !ValidateCodeChallenge("plain", strings.Repeat("a", 43), strings.Repeat("a", 43)) {
		t.Error("plain challenge must be valid")
	}
}

func TestS256Challenge(t *testing.T) {
	cv, err := newVerifier()
	if err != nil {
		t.Fatal("newVerifier:", err)
	}
	if len(cv) < 43 || len(cv) > 128 {
		t.Fatal(ErrWrongCodeVerifierLength)
	}
	cc := s256Challenge(cv)
	if got, want := len(cc), s256ChallengeLength; got != want {
		err := fmt.Errorf("S256 challenge length is %d, want %d", got, want)
		if len(cc) < 43 || len(cc) > 128 {
			err = ErrWrongCodeChallengeLength
		}
		t.Fatal(err)
	}
	if !ValidateCodeChallenge("S256", cc, cv) {
		t.Error("S256 challenge must be valid")
	}
}

// s256ChallengeLength is the expected length of the code challenge
// when S256 code challenge method is used.
const s256ChallengeLength = 43

func TestS256ChallengeLength(t *testing.T) {
	if s256ChallengeLength != base64.RawURLEncoding.EncodedLen(sha256.Size) {
		t.Errorf("internal error: s256ChallengeLength != base64.RawURLEncoding.EncodedLen(sha256.Size)")
	}
}
