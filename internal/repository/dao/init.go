package dao

import "gorm.io/gorm"

func InitTables(db *gorm.DB) error {
	return db.AutoMigrate(&UserOfDB{}, &SocialAccountOfDB{},
		&UserSignInStatOfDB{}, &UserSignInRecordOfDB{},
		&UserPointRecordOfDB{}, &EventOutboxOfDB{},
		&UserRelationOfDB{}, &NoteOfDB{}, &NoteImageOfDB{},
		&NoteLikeOfDB{}, &CommentOfDB{})
}
