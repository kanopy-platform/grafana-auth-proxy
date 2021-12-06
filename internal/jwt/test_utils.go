package jwt

import (
	"gopkg.in/square/go-jose.v2"
	josejwt "gopkg.in/square/go-jose.v2/jwt"
)

func NewTestJWTWithClaims(claims Claims) (string, error) {
	key := []byte("secret")
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}

	raw, err := josejwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", err
	}

	return raw, nil
}
