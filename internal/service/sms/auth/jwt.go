package auth

import "github.com/golang-jwt/jwt/v5"

type SMSClaims struct {
	jwt.RegisteredClaims
	TplId string `json:"tpl_id"`
}

var JWTKey = "EVa8BtXRfAMm7TcDFzWwpvhyrfQXt7c6"
