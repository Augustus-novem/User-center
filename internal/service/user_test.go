package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

var errDBDown = errors.New("db down")

type userRepoStub struct {
	createFn          func(ctx context.Context, user domain.User) error
	createAndReturnFn func(ctx context.Context, user domain.User) (domain.User, error)
	findByPhoneFn     func(ctx context.Context, phone string) (domain.User, error)
	findByIDFn        func(ctx context.Context, id int64) (domain.User, error)
	findByEmailFn     func(ctx context.Context, email string) (domain.User, error)
	updateFn          func(ctx context.Context, u domain.User) error
}

func (s *userRepoStub) Create(ctx context.Context, user domain.User) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, user)
}

func (s *userRepoStub) CreateAndReturn(ctx context.Context, user domain.User) (domain.User, error) {
	if s.createAndReturnFn == nil {
		return domain.User{}, nil
	}
	return s.createAndReturnFn(ctx, user)
}

func (s *userRepoStub) FindByPhone(ctx context.Context, phone string) (domain.User, error) {
	if s.findByPhoneFn == nil {
		return domain.User{}, nil
	}
	return s.findByPhoneFn(ctx, phone)
}

func (s *userRepoStub) FindByID(ctx context.Context, id int64) (domain.User, error) {
	if s.findByIDFn == nil {
		return domain.User{}, nil
	}
	return s.findByIDFn(ctx, id)
}

func (s *userRepoStub) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	if s.findByEmailFn == nil {
		return domain.User{}, nil
	}
	return s.findByEmailFn(ctx, email)
}

func (s *userRepoStub) Update(ctx context.Context, u domain.User) error {
	if s.updateFn == nil {
		return nil
	}
	return s.updateFn(ctx, u)
}

type socialRepoStub struct {
	createFn func(ctx context.Context, sa domain.SocialAccount) error
	findFn   func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error)
}

func (s *socialRepoStub) Create(ctx context.Context, sa domain.SocialAccount) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, sa)
}

func (s *socialRepoStub) FindByProviderAndOpenID(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
	if s.findFn == nil {
		return domain.SocialAccount{}, nil
	}
	return s.findFn(ctx, provider, openID)
}

type txStub struct {
	inTxFn func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (s *txStub) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.inTxFn == nil {
		return fn(ctx)
	}
	return s.inTxFn(ctx, fn)
}

type publishCall struct {
	topic string
	key   string
	value any
}

type publisherSpy struct {
	enabled bool
	calls   []publishCall
	fn      func(ctx context.Context, topic string, key string, value any) error
}

func (p *publisherSpy) Publish(ctx context.Context, topic string, key string, value any) error {
	p.calls = append(p.calls, publishCall{topic: topic, key: key, value: value})
	if p.fn == nil {
		return nil
	}
	return p.fn(ctx, topic, key, value)
}

func (p *publisherSpy) IsEnabled() bool {
	return p.enabled
}

func newTestUserService(userRepo repository.UserRepository) *UserServiceImpl {
	return NewUserServiceImpl(userRepo, &socialRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NoOpLogger{})
}

func TestUserServiceImpl_SignUp(t *testing.T) {
	t.Run("hashes password and publishes user.registered inside transaction", func(t *testing.T) {
		rawPassword := "hello@world123"
		inTxCalled := false
		repoCalled := false
		publisher := &publisherSpy{enabled: true}
		repo := &userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				repoCalled = true
				if !inTxCalled {
					t.Fatal("CreateAndReturn should run inside transaction")
				}
				if user.Password == rawPassword {
					t.Fatal("password should be hashed before persisting")
				}
				if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(rawPassword)); err != nil {
					t.Fatalf("password hash is invalid: %v", err)
				}
				if user.Email != "123@qq.com" {
					t.Fatalf("unexpected email: %s", user.Email)
				}
				return domain.User{Id: 1, Email: user.Email, Password: user.Password}, nil
			},
		}
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			inTxCalled = true
			return fn(ctx)
		}}
		svc := NewUserServiceImpl(repo, &socialRepoStub{}, tx, publisher, logger.NoOpLogger{})

		err := svc.SignUp(context.Background(), domain.User{Email: "123@qq.com", Password: rawPassword})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repoCalled {
			t.Fatal("CreateAndReturn should be called")
		}
		if len(publisher.calls) != 1 {
			t.Fatalf("want publisher called once, got %d", len(publisher.calls))
		}
		call := publisher.calls[0]
		if call.topic != events.TopicUserRegistered {
			t.Fatalf("unexpected topic: %s", call.topic)
		}
		if call.key != events.UserIDKey(1) {
			t.Fatalf("unexpected key: %s", call.key)
		}
		evt, ok := call.value.(events.UserRegisteredEvent)
		if !ok {
			t.Fatalf("unexpected event type: %T", call.value)
		}
		if evt.UserID != 1 || evt.Email != "123@qq.com" {
			t.Fatalf("unexpected event payload: %+v", evt)
		}
	})

	t.Run("publisher disabled skips event publishing", func(t *testing.T) {
		publisher := &publisherSpy{enabled: false}
		repo := &userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				return domain.User{Id: 2, Email: user.Email, Password: user.Password}, nil
			},
		}
		svc := NewUserServiceImpl(repo, &socialRepoStub{}, &txStub{}, publisher, logger.NoOpLogger{})

		err := svc.SignUp(context.Background(), domain.User{Email: "no-publish@qq.com", Password: "123456Aa!"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("publisher should not be called when disabled, got %d calls", len(publisher.calls))
		}
	})
}

func TestUserServiceImpl_Login(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("hello@world123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to create bcrypt hash: %v", err)
	}

	tests := []struct {
		name    string
		repo    repository.UserRepository
		email   string
		pwd     string
		wantErr error
		wantID  int64
	}{
		{
			name: "login success",
			repo: &userRepoStub{findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				return domain.User{Id: 12, Email: email, Password: string(hashed)}, nil
			}},
			email:  "123@qq.com",
			pwd:    "hello@world123",
			wantID: 12,
		},
		{
			name: "user not found",
			repo: &userRepoStub{findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				return domain.User{}, repository.ErrUserNotFound
			}},
			email:   "123@qq.com",
			pwd:     "hello@world123",
			wantErr: ErrInvalidUserOrPassword,
		},
		{
			name: "wrong password",
			repo: &userRepoStub{findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				return domain.User{Id: 12, Email: email, Password: string(hashed)}, nil
			}},
			email:   "123@qq.com",
			pwd:     "bad-password",
			wantErr: ErrInvalidUserOrPassword,
		},
		{
			name: "repository error bubbles up",
			repo: &userRepoStub{findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				return domain.User{}, errDBDown
			}},
			email:   "123@qq.com",
			pwd:     "hello@world123",
			wantErr: errDBDown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestUserService(tc.repo)
			user, err := svc.Login(context.Background(), tc.email, tc.pwd)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want err %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.Id != tc.wantID {
				t.Fatalf("want user id %d, got %d", tc.wantID, user.Id)
			}
		})
	}
}

func TestUserServiceImpl_FindOrCreate(t *testing.T) {
	t.Run("existing user returns immediately without opening transaction", func(t *testing.T) {
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			t.Fatal("transaction should not be opened for existing user")
			return nil
		}}
		svc := NewUserServiceImpl(&userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				return domain.User{Id: 1, Phone: phone}, nil
			},
		}, &socialRepoStub{}, tx, &publisherSpy{enabled: true}, logger.NoOpLogger{})

		user, err := svc.FindOrCreate(context.Background(), "13800138000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 1 {
			t.Fatalf("want id 1, got %d", user.Id)
		}
	})

	t.Run("create new user when phone not found", func(t *testing.T) {
		findCalls := 0
		publisher := &publisherSpy{enabled: true}
		inTxCalled := false
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				findCalls++
				return domain.User{}, repository.ErrUserNotFound
			},
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				if !inTxCalled {
					t.Fatal("CreateAndReturn should run inside transaction")
				}
				if user.Phone != "13800138001" {
					t.Fatalf("unexpected phone: %s", user.Phone)
				}
				return domain.User{Id: 2, Phone: user.Phone}, nil
			},
		}
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			inTxCalled = true
			return fn(ctx)
		}}
		svc := NewUserServiceImpl(repo, &socialRepoStub{}, tx, publisher, logger.NoOpLogger{})

		user, err := svc.FindOrCreate(context.Background(), "13800138001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 2 {
			t.Fatalf("want id 2, got %d", user.Id)
		}
		if findCalls != 1 {
			t.Fatalf("want initial lookup once, got %d", findCalls)
		}
		if len(publisher.calls) != 1 {
			t.Fatalf("want publisher called once, got %d", len(publisher.calls))
		}
	})

	t.Run("duplicate create falls back to querying existing user", func(t *testing.T) {
		findCalls := 0
		publisher := &publisherSpy{enabled: true}
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				findCalls++
				if findCalls == 1 {
					return domain.User{}, repository.ErrUserNotFound
				}
				return domain.User{Id: 3, Phone: phone}, nil
			},
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				return domain.User{}, repository.ErrUserDuplicate
			},
		}
		svc := NewUserServiceImpl(repo, &socialRepoStub{}, &txStub{}, publisher, logger.NoOpLogger{})

		user, err := svc.FindOrCreate(context.Background(), "13800138002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 3 {
			t.Fatalf("want id 3, got %d", user.Id)
		}
		if findCalls != 2 {
			t.Fatalf("want phone lookup twice, got %d", findCalls)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("publisher should not be called on duplicate create, got %d", len(publisher.calls))
		}
	})
}

func TestUserServiceImpl_FindOrCreateByWechat(t *testing.T) {
	t.Run("existing social account returns existing user without opening transaction", func(t *testing.T) {
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			t.Fatal("transaction should not be opened for existing social account")
			return nil
		}}
		userRepo := &userRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
				if id != 99 {
					t.Fatalf("unexpected user id: %d", id)
				}
				return domain.User{Id: 99, Email: "wx@qq.com"}, nil
			},
		}
		socialRepo := &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				if provider != domain.OAuthProviderWechat || openID != "openid-1" {
					t.Fatalf("unexpected provider/openid: %s %s", provider, openID)
				}
				return domain.SocialAccount{UserId: 99}, nil
			},
		}
		svc := NewUserServiceImpl(userRepo, socialRepo, tx, &publisherSpy{enabled: true}, logger.NoOpLogger{})

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 99 {
			t.Fatalf("want id 99, got %d", user.Id)
		}
	})

	t.Run("first wechat login creates local user, social binding and event", func(t *testing.T) {
		inTxCalled := false
		publisher := &publisherSpy{enabled: true}
		userRepo := &userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				if !inTxCalled {
					t.Fatal("CreateAndReturn should run inside transaction")
				}
				if user != (domain.User{}) {
					t.Fatalf("wechat first login should create empty local user, got %+v", user)
				}
				return domain.User{Id: 123}, nil
			},
		}
		socialRepo := &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				if sa.UserId != 123 {
					t.Fatalf("want social account bind to user 123, got %d", sa.UserId)
				}
				if sa.Provider != domain.OAuthProviderWechat || sa.OpenId != "openid-2" || sa.UnionId != "union-2" {
					t.Fatalf("unexpected social account: %+v", sa)
				}
				return nil
			},
		}
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			inTxCalled = true
			return fn(ctx)
		}}
		svc := NewUserServiceImpl(userRepo, socialRepo, tx, publisher, logger.NoOpLogger{})

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-2", UnionId: "union-2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 123 {
			t.Fatalf("want id 123, got %d", user.Id)
		}
		if len(publisher.calls) != 1 {
			t.Fatalf("want publisher called once, got %d", len(publisher.calls))
		}
	})

	t.Run("duplicate social binding falls back to existing user", func(t *testing.T) {
		findCalls := 0
		publisher := &publisherSpy{enabled: true}
		userRepo := &userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				return domain.User{Id: 111}, nil
			},
			findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
				if id != 999 {
					t.Fatalf("want query existing user id 999, got %d", id)
				}
				return domain.User{Id: 999, Email: "existing@qq.com"}, nil
			},
		}
		socialRepo := &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				findCalls++
				if findCalls == 1 {
					return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
				}
				return domain.SocialAccount{UserId: 999, Provider: domain.OAuthProviderWechat, OpenId: "openid-dup", UnionId: "union-dup"}, nil
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				return repository.ErrSocialAccountDuplicated
			},
		}
		svc := NewUserServiceImpl(userRepo, socialRepo, &txStub{}, publisher, logger.NoOpLogger{})

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-dup", UnionId: "union-dup"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 999 {
			t.Fatalf("want existing user id 999, got %d", user.Id)
		}
		if findCalls != 2 {
			t.Fatalf("want social lookup twice, got %d", findCalls)
		}
		if len(publisher.calls) != 0 {
			t.Fatalf("publisher should not be called on duplicate social binding, got %d", len(publisher.calls))
		}
	})
}
