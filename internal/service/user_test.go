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
	createFn      func(ctx context.Context, user domain.User) error
	findByPhoneFn func(ctx context.Context, phone string) (domain.User, error)
	findByIDFn    func(ctx context.Context, id int64) (domain.User, error)
	findByEmailFn func(ctx context.Context, email string) (domain.User, error)
}

func (s *userRepoStub) Create(ctx context.Context, user domain.User) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, user)
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
	svc := NewUserServiceImpl(repo)

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
			svc := NewUserServiceImpl(tc.repo)
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
		svc := NewUserServiceImpl(repo)
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
		svc := NewUserServiceImpl(repo)
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
		svc := NewUserServiceImpl(repo)
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
		svc := NewUserServiceImpl(repo)
		_, err := svc.FindOrCreate(context.Background(), "13800138003")
		if err == nil || err.Error() != "db write failed" {
			t.Fatalf("want db write failed, got %v", err)
		}
	})
}
