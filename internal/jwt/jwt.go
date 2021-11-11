package jwt

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/square/go-jose.v2/jwt"
)

// Claims is a wrapper of jwt.Claims with added attributes
type Claims struct {
	jwt.Claims
	Groups []string `json:"groups"`
	Email  string   `json:"email"`
}

// TokenClaims returns Claims from a jwt token in raw base64 format
func TokenClaims(rawToken string) (*Claims, error) {
	token, err := jwt.ParseSigned(rawToken)
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
