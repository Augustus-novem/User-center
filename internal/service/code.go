package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"user-center/internal/repository"
	"user-center/internal/service/sms"
)

const codeTplId = "1877556"

var ErrCodeSendTooMany = repository.ErrCodeSendTooMany

type CodeService interface {
	Send(ctx context.Context, biz, phone string) error
	Verify(ctx context.Context, biz, phone, inputCode string) (bool, error)
}

type SMSCodeService struct {
	CodeRepo repository.CodeRepository
	sms      sms.Service
}

func NewSMSCodeService(codeRepository repository.CodeRepository, sms sms.Service) *SMSCodeService {
	return &SMSCodeService{
		CodeRepo: codeRepository,
		sms:      sms}
}

func (cs *SMSCodeService) Verify(ctx context.Context,
	biz, phone, code string) (bool, error) {
	ok, err := cs.CodeRepo.Verify(ctx, biz, phone, code)
	if errors.Is(err, repository.ErrCodeVerifyTooManyTimes) {
		//记录
		return false, nil
	}
	return ok, err
}

func (cs *SMSCodeService) Send(ctx context.Context, biz, phone string) error {
	code, err := cs.generate()
	if err != nil {
		return fmt.Errorf("generate SMS code: %w", err)
	}
	err = cs.CodeRepo.Store(ctx, biz, phone, code)
	if err != nil {
		return err
	}
	err = cs.sms.Send(ctx, codeTplId, []string{code}, phone)
	return err
}

func (cs *SMSCodeService) generate() (string, error) {
	code, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", code.Int64()), nil
}
