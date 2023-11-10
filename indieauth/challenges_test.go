package indieauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlainChallenge(t *testing.T) {
	// Generate a code_verifier within the allowed minimum length of 43 characters and maximum
	// length of 128 characters, per https://datatracker.ietf.org/doc/html/rfc7636#section-4.1.
	require.True(t, ValidateCodeChallenge("plain", strings.Repeat("a", 43), strings.Repeat("a", 43)))
}

func TestS256Challenge(t *testing.T) {
	cv, err := newVerifier()
	require.NoError(t, err)
	require.False(t, len(cv) < 43 || len(cv) > 128)

	cc := s256Challenge(cv)
	if got, want := len(cc), s256ChallengeLength; got != want {
		err := fmt.Errorf("S256 challenge length is %d, want %d", got, want)
		if len(cc) < 43 || len(cc) > 128 {
			err = ErrWrongCodeChallengeLength
		}
		t.Fatal(err)
	}
	require.True(t, ValidateCodeChallenge("S256", cc, cv))
}

// s256ChallengeLength is the expected length of the code challenge
// when S256 code challenge method is used.
const s256ChallengeLength = 43

func TestS256ChallengeLength(t *testing.T) {
	require.Equal(t, s256ChallengeLength, base64.RawURLEncoding.EncodedLen(sha256.Size))
}
