package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

type userDAOStub struct {
	insertFn             func(ctx context.Context, user dao.UserOfDB) error
	insertRetFn          func(ctx context.Context, user dao.UserOfDB) (dao.UserOfDB, error)
	findByEmailFn        func(ctx context.Context, email string) (dao.UserOfDB, error)
	findByIDFn           func(ctx context.Context, id int64) (dao.UserOfDB, error)
	findByPhoneFn        func(ctx context.Context, phone string) (dao.UserOfDB, error)
	updateNonSensitiveFn func(ctx context.Context, user dao.UserOfDB) error
}

func (s *userDAOStub) Insert(ctx context.Context, user dao.UserOfDB) error {
	if s.insertFn == nil {
		return nil
	}
	return s.insertFn(ctx, user)
}

func (s *userDAOStub) InsertAndReturn(ctx context.Context, user dao.UserOfDB) (dao.UserOfDB, error) {
	if s.insertRetFn == nil {
		return user, nil
	}
	return s.insertRetFn(ctx, user)
}

func (s *userDAOStub) FindByEmail(ctx context.Context, email string) (dao.UserOfDB, error) {
	if s.findByEmailFn == nil {
		return dao.UserOfDB{}, nil
	}
	return s.findByEmailFn(ctx, email)
}

func (s *userDAOStub) FindById(ctx context.Context, id int64) (dao.UserOfDB, error) {
	if s.findByIDFn == nil {
		return dao.UserOfDB{}, nil
	}
	return s.findByIDFn(ctx, id)
}

func (s *userDAOStub) FindByPhone(ctx context.Context, phone string) (dao.UserOfDB, error) {
	if s.findByPhoneFn == nil {
		return dao.UserOfDB{}, nil
	}
	return s.findByPhoneFn(ctx, phone)
}

func (s *userDAOStub) UpdateNonSensitive(ctx context.Context, user dao.UserOfDB) error {
	if s.updateNonSensitiveFn == nil {
		return nil
	}
	return s.updateNonSensitiveFn(ctx, user)
}

type userCacheStub struct {
	getFn    func(ctx context.Context, id int64) (domain.User, error)
	setFn    func(ctx context.Context, user domain.User) error
	deleteFn func(ctx context.Context, id int64) error
}

func (s *userCacheStub) Get(ctx context.Context, id int64) (domain.User, error) {
	if s.getFn == nil {
		return domain.User{}, nil
	}
	return s.getFn(ctx, id)
}

func (s *userCacheStub) Set(ctx context.Context, user domain.User) error {
	if s.setFn == nil {
		return nil
	}
	return s.setFn(ctx, user)
}

func (s *userCacheStub) Delete(ctx context.Context, id int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, id)
}

func TestCachedUserRepository_FindByID(t *testing.T) {
	t.Parallel()

	t.Run("cache hit", func(t *testing.T) {
		t.Parallel()
		repo := NewCachedUserRepository(&userDAOStub{}, &userCacheStub{
			getFn: func(ctx context.Context, id int64) (domain.User, error) {
				return domain.User{Id: id, Email: "cached@qq.com"}, nil
			},
			setFn: func(ctx context.Context, user domain.User) error {
				t.Fatal("cache.Set should not be called on cache hit")
				return nil
			},
		})

		user, err := repo.FindByID(context.Background(), 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "cached@qq.com" {
			t.Fatalf("unexpected user: %+v", user)
		}
	})

	t.Run("cache miss then load from dao and write cache", func(t *testing.T) {
		t.Parallel()
		cacheWritten := false
		repo := NewCachedUserRepository(&userDAOStub{
			findByIDFn: func(ctx context.Context, id int64) (dao.UserOfDB, error) {
				return dao.UserOfDB{
					Id:       id,
					Email:    sql.NullString{String: "dao@qq.com", Valid: true},
					Phone:    sql.NullString{String: "13800138000", Valid: true},
					Password: "hashed",
				}, nil
			},
		}, &userCacheStub{
			getFn: func(ctx context.Context, id int64) (domain.User, error) {
				return domain.User{}, errors.New("cache miss")
			},
			setFn: func(ctx context.Context, user domain.User) error {
				cacheWritten = true
				if user.Id != 13 || user.Email != "dao@qq.com" {
					t.Fatalf("unexpected user written to cache: %+v", user)
				}
				return nil
			},
		})

		user, err := repo.FindByID(context.Background(), 13)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "dao@qq.com" || user.Phone != "13800138000" {
			t.Fatalf("unexpected user: %+v", user)
		}
		if !cacheWritten {
			t.Fatal("cache.Set should be called after dao hit")
		}
	})

	t.Run("dao not found", func(t *testing.T) {
		t.Parallel()
		repo := NewCachedUserRepository(&userDAOStub{
			findByIDFn: func(ctx context.Context, id int64) (dao.UserOfDB, error) {
				return dao.UserOfDB{}, dao.ErrDataNotFound
			},
		}, &userCacheStub{
			getFn: func(ctx context.Context, id int64) (domain.User, error) {
				return domain.User{}, errors.New("cache miss")
			},
		})

		_, err := repo.FindByID(context.Background(), 14)
		if !errors.Is(err, dao.ErrDataNotFound) {
			t.Fatalf("want err %v, got %v", dao.ErrDataNotFound, err)
		}
	})
}

func TestCachedUserRepository_Create_MapsDomainToDAO(t *testing.T) {
	t.Parallel()

	repo := NewCachedUserRepository(&userDAOStub{
		insertFn: func(ctx context.Context, user dao.UserOfDB) error {
			if !user.Email.Valid || user.Email.String != "123@qq.com" {
				t.Fatalf("unexpected email mapping: %+v", user.Email)
			}
			if !user.Phone.Valid || user.Phone.String != "13800138000" {
				t.Fatalf("unexpected phone mapping: %+v", user.Phone)
			}
			if user.Password != "hashed" {
				t.Fatalf("unexpected password mapping: %s", user.Password)
			}
			return nil
		},
	}, &userCacheStub{})

	err := repo.Create(context.Background(), domain.User{
		Email:    "123@qq.com",
		Phone:    "13800138000",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
