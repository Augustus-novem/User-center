package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
)

var ErrAccountNotFound = gorm.ErrRecordNotFound

type SocialAccountDAO interface {
	Insert(ctx context.Context, sa SocialAccountOfDB) error
	FindByProviderAndOpenID(ctx context.Context, provider string, openID string) (SocialAccountOfDB, error)
	FindByProviderAndUnionID(ctx context.Context, provider string, unionID string) (SocialAccountOfDB, error)
}

type GORMSocialAccountDAO struct {
	db *gorm.DB
}

func NewGormSocialAccountDAO(db *gorm.DB) *GORMSocialAccountDAO {
	return &GORMSocialAccountDAO{
		db: db,
	}
}

func (dao *GORMSocialAccountDAO) Insert(ctx context.Context, sa SocialAccountOfDB) error {
	now := time.Now().UnixMilli()
	sa.Ctime = now
	sa.Utime = now
	return dbFromCtx(ctx, dao.db).Create(&sa).Error
}

func (dao *GORMSocialAccountDAO) FindByProviderAndOpenID(ctx context.Context, provider string, openID string) (SocialAccountOfDB, error) {
	var sa SocialAccountOfDB
	err := dbFromCtx(ctx, dao.db).
		Where("provider = ? AND open_id = ?", provider, openID).
		First(&sa).Error
	return sa, err
}

func (dao *GORMSocialAccountDAO) FindByProviderAndUnionID(ctx context.Context, provider string, unionID string) (SocialAccountOfDB, error) {
	var sa SocialAccountOfDB
	err := dbFromCtx(ctx, dao.db).
		Where("provider = ? AND union_id = ?", provider, unionID).
		First(&sa).Error
	return sa, err
}

type SocialAccountOfDB struct {
	Id int64 `gorm:"primaryKey,autoIncrement"`

	// 联合唯一索引 1: 确保【同一个系统用户，针对同一个第三方平台】只能绑定一个有效账号
	UserId int64 `gorm:"uniqueIndex:uidx_user_provider,priority:1;not null"`

	// 复合索引的第一个字段，同时参与两个联合唯一索引
	Provider string `gorm:"type:varchar(32);not null;default:'';uniqueIndex:uidx_provider_openid,priority:1;uniqueIndex:uidx_user_provider,priority:2"`

	// 联合唯一索引 2: 确保【同一个第三方账号】在系统中只能有一次有效绑定
	OpenId string `gorm:"type:varchar(128);not null;default:'';uniqueIndex:uidx_provider_openid,priority:2"`

	// 加上 not null，没数据时默认为空字符串，避免 NULL 坑
	UnionId string `gorm:"type:varchar(128);not null;default:'';index:idx_union_id"`

	Ctime int64 `gorm:"autoCreateTime:milli"` // GORM 可以自动帮你管理这两个时间
	Utime int64 `gorm:"autoUpdateTime:milli"`

	// 核心改造：用删除时间戳代替 bool
	// 将 DelTime 放入两个唯一索引的末尾。
	// 未删除时值为 0（正常防重）；删除时写入当前时间戳（解除唯一性封印）
	DelTime int64 `gorm:"uniqueIndex:uidx_provider_openid,priority:3;uniqueIndex:uidx_user_provider,priority:3;not null;default:0"`
}
