package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
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
	code := cs.generate()
	err := cs.CodeRepo.Store(ctx, biz, phone, code)
	if err != nil {
		return err
	}
	err = cs.sms.Send(ctx, codeTplId, []string{code}, phone)
	return err
}

func (cs *SMSCodeService) generate() string {
	code := rand.Intn(1000000)
	return fmt.Sprintf("%06d", code)
}
