package jwt

import (
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	log "github.com/sirupsen/logrus"
)

var signatureAlgorithms = []jose.SignatureAlgorithm{
	jose.HS256, jose.HS384, jose.HS512,
	jose.RS256, jose.RS384, jose.RS512,
	jose.ES256, jose.ES384, jose.ES512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.EdDSA,
}

// Claims is a wrapper of jwt.Claims with added attributes
type Claims struct {
	jwt.Claims
	Groups []string `json:"groups"`
	Email  string   `json:"email"`
}

// TokenClaims returns Claims from a jwt token in raw base64 format
func TokenClaims(rawToken string) (*Claims, error) {
	token, err := jwt.ParseSigned(rawToken, signatureAlgorithms)
	if err != nil {
		log.Error("Error when parsing the token, ", err)
		return nil, err
	}

	out := &Claims{}
	if err := token.UnsafeClaimsWithoutVerification(out); err != nil {
		log.Error("Error when getting Claims from token, ", err)
		return nil, err
	}

	return out, nil
}
