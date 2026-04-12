package domain

import "time"

type User struct {
	Id       int64
	Email    string
	Phone    string
	Password string
	NickName string
	AboutMe  string
	Birthday time.Time
	Ctime    int64
}
