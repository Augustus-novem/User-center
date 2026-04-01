package service

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var errDBDown = errors.New("db down")

type userRepoStub struct {
	createFn          func(ctx context.Context, user domain.User) error
	createAndReturnFn func(ctx context.Context, user domain.User) (domain.User, error)
	findByPhoneFn     func(ctx context.Context, phone string) (domain.User, error)
	findByIDFn        func(ctx context.Context, id int64) (domain.User, error)
	findByEmailFn     func(ctx context.Context, email string) (domain.User, error)
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

func newTestUserService(userRepo repository.UserRepository) *UserServiceImpl {
	return NewUserServiceImpl(userRepo, &socialRepoStub{}, &txStub{})
}

func TestUserServiceImpl_SignUp_HashesPasswordBeforeCreate(t *testing.T) {
	t.Parallel()

	rawPassword := "hello@world123"
	repo := &userRepoStub{
		createFn: func(ctx context.Context, user domain.User) error {
			if user.Password == rawPassword {
				t.Fatalf("password should be hashed before repository.Create")
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(rawPassword)); err != nil {
				t.Fatalf("password hash is invalid: %v", err)
			}
			if user.Email != "123@qq.com" {
				t.Fatalf("unexpected email: %s", user.Email)
			}
			return nil
		},
	}
	svc := newTestUserService(repo)

	err := svc.SignUp(context.Background(), domain.User{
		Email:    "123@qq.com",
		Password: rawPassword,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserServiceImpl_Login(t *testing.T) {
	t.Parallel()

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
			repo: &userRepoStub{
				findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{Id: 12, Email: email, Password: string(hashed)}, nil
				},
			},
			email:  "123@qq.com",
			pwd:    "hello@world123",
			wantID: 12,
		},
		{
			name: "user not found",
			repo: &userRepoStub{
				findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, repository.ErrUserNotFound
				},
			},
			email:   "123@qq.com",
			pwd:     "hello@world123",
			wantErr: ErrInvalidUserOrPassword,
		},
		{
			name: "wrong password",
			repo: &userRepoStub{
				findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{Id: 12, Email: email, Password: string(hashed)}, nil
				},
			},
			email:   "123@qq.com",
			pwd:     "bad-password",
			wantErr: ErrInvalidUserOrPassword,
		},
		{
			name: "repository error",
			repo: &userRepoStub{
				findByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
					return domain.User{}, errDBDown
				},
			},
			email:   "123@qq.com",
			pwd:     "hello@world123",
			wantErr: errDBDown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()

	t.Run("existing user", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				return domain.User{Id: 1, Phone: phone}, nil
			},
		}
		svc := newTestUserService(repo)
		user, err := svc.FindOrCreate(context.Background(), "13800138000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 1 {
			t.Fatalf("want id 1, got %d", user.Id)
		}
	})

	t.Run("create new user after not found", func(t *testing.T) {
		t.Parallel()
		calls := 0
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				calls++
				if calls == 1 {
					return domain.User{}, repository.ErrUserNotFound
				}
				return domain.User{Id: 2, Phone: phone}, nil
			},
			createFn: func(ctx context.Context, user domain.User) error {
				if user.Phone != "13800138001" {
					t.Fatalf("unexpected phone: %s", user.Phone)
				}
				return nil
			},
		}
		svc := newTestUserService(repo)
		user, err := svc.FindOrCreate(context.Background(), "13800138001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 2 {
			t.Fatalf("want id 2, got %d", user.Id)
		}
	})

	t.Run("create returns duplicate then query again", func(t *testing.T) {
		t.Parallel()
		calls := 0
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				calls++
				if calls == 1 {
					return domain.User{}, repository.ErrUserNotFound
				}
				return domain.User{Id: 3, Phone: phone}, nil
			},
			createFn: func(ctx context.Context, user domain.User) error {
				return repository.ErrUserDuplicate
			},
		}
		svc := newTestUserService(repo)
		user, err := svc.FindOrCreate(context.Background(), "13800138002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 3 {
			t.Fatalf("want id 3, got %d", user.Id)
		}
	})

	t.Run("create returns system error", func(t *testing.T) {
		t.Parallel()
		repo := &userRepoStub{
			findByPhoneFn: func(ctx context.Context, phone string) (domain.User, error) {
				return domain.User{}, repository.ErrUserNotFound
			},
			createFn: func(ctx context.Context, user domain.User) error {
				return errors.New("db write failed")
			},
		}
		svc := newTestUserService(repo)
		_, err := svc.FindOrCreate(context.Background(), "13800138003")
		if err == nil || err.Error() != "db write failed" {
			t.Fatalf("want db write failed, got %v", err)
		}
	})
}

func TestUserServiceImpl_FindOrCreateByWechat(t *testing.T) {
	t.Parallel()

	t.Run("existing social account returns existing user", func(t *testing.T) {
		t.Parallel()
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
		tx := &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			t.Fatal("transaction should not be opened for existing social account")
			return nil
		}}
		svc := NewUserServiceImpl(userRepo, socialRepo, tx)

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 99 {
			t.Fatalf("want id 99, got %d", user.Id)
		}
	})

	t.Run("create new user and social account in transaction", func(t *testing.T) {
		t.Parallel()
		calledCreateSocial := false
		userRepo := &userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				if user != (domain.User{}) {
					t.Fatalf("wechat first-login should create empty local user, got %+v", user)
				}
				return domain.User{Id: 123}, nil
			},
		}
		socialRepo := &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				calledCreateSocial = true
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
			return fn(ctx)
		}}
		svc := NewUserServiceImpl(userRepo, socialRepo, tx)

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{
			OpenId:  "openid-2",
			UnionId: "union-2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 123 {
			t.Fatalf("want id 123, got %d", user.Id)
		}
		if !calledCreateSocial {
			t.Fatal("social account should be created inside transaction")
		}
	})

	t.Run("social repository unexpected error bubbles up", func(t *testing.T) {
		t.Parallel()
		svc := NewUserServiceImpl(&userRepoStub{}, &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				return domain.SocialAccount{}, errDBDown
			},
		}, &txStub{})

		_, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-3"})
		if !errors.Is(err, errDBDown) {
			t.Fatalf("want err %v, got %v", errDBDown, err)
		}
	})

	t.Run("social account create error rolls back transaction", func(t *testing.T) {
		t.Parallel()
		svc := NewUserServiceImpl(&userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				return domain.User{Id: 321}, nil
			},
		}, &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				return errors.New("bind social account failed")
			},
		}, &txStub{inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		}})

		_, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{OpenId: "openid-4"})
		if err == nil || err.Error() != "bind social account failed" {
			t.Fatalf("want bind social account failed, got %v", err)
		}
	})
	t.Run("duplicate social account then query existing user", func(t *testing.T) {
		t.Parallel()

		findCalls := 0
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
				if provider != domain.OAuthProviderWechat {
					t.Fatalf("unexpected provider: %v", provider)
				}
				if openID != "openid-dup" {
					t.Fatalf("unexpected openid: %s", openID)
				}
				// 第一次查：还没绑定
				if findCalls == 1 {
					return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
				}
				// 第二次查：并发下别人已经创建好了绑定
				return domain.SocialAccount{
					UserId:   999,
					Provider: domain.OAuthProviderWechat,
					OpenId:   "openid-dup",
					UnionId:  "union-dup",
				}, nil
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				if sa.UserId != 111 {
					t.Fatalf("want new created user id 111, got %d", sa.UserId)
				}
				if sa.Provider != domain.OAuthProviderWechat || sa.OpenId != "openid-dup" || sa.UnionId != "union-dup" {
					t.Fatalf("unexpected social account: %+v", sa)
				}
				// 模拟并发下唯一索引冲突
				return repository.ErrSocialAccountDuplicated
			},
		}
		tx := &txStub{
			inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		}

		svc := NewUserServiceImpl(userRepo, socialRepo, tx)

		user, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{
			OpenId:  "openid-dup",
			UnionId: "union-dup",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Id != 999 {
			t.Fatalf("want existing user id 999, got %d", user.Id)
		}
		if findCalls != 2 {
			t.Fatalf("want find social account called twice, got %d", findCalls)
		}
	})
	t.Run("duplicate social account but requery fails", func(t *testing.T) {
		t.Parallel()

		findCalls := 0
		svc := NewUserServiceImpl(&userRepoStub{
			createAndReturnFn: func(ctx context.Context, user domain.User) (domain.User, error) {
				return domain.User{Id: 111}, nil
			},
		}, &socialRepoStub{
			findFn: func(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
				findCalls++
				if findCalls == 1 {
					return domain.SocialAccount{}, repository.ErrSocialAccountNotFound
				}
				return domain.SocialAccount{}, errDBDown
			},
			createFn: func(ctx context.Context, sa domain.SocialAccount) error {
				return repository.ErrSocialAccountDuplicated
			},
		}, &txStub{
			inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		})

		_, err := svc.FindOrCreateByWechat(context.Background(), domain.SocialAccount{
			OpenId: "openid-dup-2",
		})
		if !errors.Is(err, errDBDown) {
			t.Fatalf("want err %v, got %v", errDBDown, err)
		}
	})
}
