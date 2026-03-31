package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"user-center/internal/repository"
)

type codeRepoStub struct {
	storeFn  func(ctx context.Context, biz, phone, code string) error
	verifyFn func(ctx context.Context, biz, phone, inputCode string) (bool, error)
}

func (s *codeRepoStub) Store(ctx context.Context, biz, phone, code string) error {
	if s.storeFn == nil {
		return nil
	}
	return s.storeFn(ctx, biz, phone, code)
}

func (s *codeRepoStub) Verify(ctx context.Context, biz, phone, inputCode string) (bool, error) {
	if s.verifyFn == nil {
		return false, nil
	}
	return s.verifyFn(ctx, biz, phone, inputCode)
}

type smsServiceStub struct {
	sendFn func(ctx context.Context, tplId string, args []string, numbers ...string) error
}

func (s *smsServiceStub) Send(ctx context.Context, tplId string, args []string, numbers ...string) error {
	if s.sendFn == nil {
		return nil
	}
	return s.sendFn(ctx, tplId, args, numbers...)
}

func TestSMSCodeService_Send(t *testing.T) {
	t.Parallel()

	t.Run("send success", func(t *testing.T) {
		t.Parallel()
		repo := &codeRepoStub{
			storeFn: func(ctx context.Context, biz, phone, code string) error {
				if biz != "login" || phone != "13800138000" {
					t.Fatalf("unexpected args: biz=%s phone=%s", biz, phone)
				}
				matched, _ := regexp.MatchString(`^\d{6}$`, code)
				if !matched {
					t.Fatalf("generated code should be 6 digits, got %s", code)
				}
				return nil
			},
		}
		smsSvc := &smsServiceStub{
			sendFn: func(ctx context.Context, tplId string, args []string, numbers ...string) error {
				if tplId != codeTplId {
					t.Fatalf("unexpected tplId: %s", tplId)
				}
				if len(args) != 1 {
					t.Fatalf("unexpected sms args length: %d", len(args))
				}
				matched, _ := regexp.MatchString(`^\d{6}$`, args[0])
				if !matched {
					t.Fatalf("sms code should be 6 digits, got %s", args[0])
				}
				if len(numbers) != 1 || numbers[0] != "13800138000" {
					t.Fatalf("unexpected sms numbers: %v", numbers)
				}
				return nil
			},
		}
		svc := NewSMSCodeService(repo, smsSvc)
		if err := svc.Send(context.Background(), "login", "13800138000"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repository rejects frequent sends", func(t *testing.T) {
		t.Parallel()
		repo := &codeRepoStub{
			storeFn: func(ctx context.Context, biz, phone, code string) error {
				return ErrCodeSendTooMany
			},
		}
		smsSvc := &smsServiceStub{
			sendFn: func(ctx context.Context, tplId string, args []string, numbers ...string) error {
				t.Fatal("sms should not be called when repository.Store fails")
				return nil
			},
		}
		svc := NewSMSCodeService(repo, smsSvc)
		err := svc.Send(context.Background(), "login", "13800138000")
		if !errors.Is(err, ErrCodeSendTooMany) {
			t.Fatalf("want err %v, got %v", ErrCodeSendTooMany, err)
		}
	})

	t.Run("sms provider error bubbles up", func(t *testing.T) {
		t.Parallel()
		repo := &codeRepoStub{}
		smsSvc := &smsServiceStub{
			sendFn: func(ctx context.Context, tplId string, args []string, numbers ...string) error {
				return errors.New("sms provider down")
			},
		}
		svc := NewSMSCodeService(repo, smsSvc)
		err := svc.Send(context.Background(), "login", "13800138000")
		if err == nil || err.Error() != "sms provider down" {
			t.Fatalf("want sms provider down, got %v", err)
		}
	})
}

func TestSMSCodeService_Verify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repo    *codeRepoStub
		wantOK  bool
		wantNil bool
		wantErr string
	}{
		{
			name: "verify success",
			repo: &codeRepoStub{
				verifyFn: func(ctx context.Context, biz, phone, inputCode string) (bool, error) {
					return true, nil
				},
			},
			wantOK:  true,
			wantNil: true,
		},
		{
			name: "too many verify attempts returns false nil",
			repo: &codeRepoStub{
				verifyFn: func(ctx context.Context, biz, phone, inputCode string) (bool, error) {
					return false, repository.ErrCodeVerifyTooManyTimes
				},
			},
			wantOK:  false,
			wantNil: true,
		},
		{
			name: "system error bubbles up",
			repo: &codeRepoStub{
				verifyFn: func(ctx context.Context, biz, phone, inputCode string) (bool, error) {
					return false, errors.New("redis down")
				},
			},
			wantOK:  false,
			wantErr: "redis down",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewSMSCodeService(tc.repo, &smsServiceStub{})
			ok, err := svc.Verify(context.Background(), "login", "13800138000", "123456")
			if ok != tc.wantOK {
				t.Fatalf("want ok=%v, got %v", tc.wantOK, ok)
			}
			if tc.wantNil {
				if err != nil {
					t.Fatalf("want nil err, got %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("want err %q, got %v", tc.wantErr, err)
			}
		})
	}
}
