package auth

import (
	"context"
	"time"
	"user-center/internal/service/sms"

	"github.com/golang-jwt/jwt/v5"
)

type SMSService struct {
	svc sms.Service
	key []byte
}

func NewService(svc sms.Service) *SMSService {
	return &SMSService{
		svc: svc,
		key: []byte(JWTKey),
	}
}

func (s *SMSService) GenToken(tplId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, SMSClaims{
		TplId: tplId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 10)),
		},
	})
	return token.SignedString(s.key)
}

func (s *SMSService) Send(ctx context.Context,
	tplToken string, args []string, numbers ...string) error {
	var sc SMSClaims
	_, err := jwt.ParseWithClaims(tplToken, &sc, func(token *jwt.Token) (interface{}, error) {
		return s.key, nil
	})
	if err != nil {
		return err
	}
	return s.svc.Send(ctx, sc.TplId, args, numbers...)
}
