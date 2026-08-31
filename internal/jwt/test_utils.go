package jwt

import (
	jose "github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

func NewTestJWTWithClaims(claims Claims) (string, error) {
	// go-jose v4 requires HMAC keys to be at least as long as the hash output (32 bytes for HS256)
	key := []byte("test-secret-key-32-bytes-long!!!")
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}

	raw, err := josejwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		return "", err
	}

	return raw, nil
}
