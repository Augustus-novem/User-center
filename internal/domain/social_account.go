package domain

type OAuthProvider string

const (
	OAuthProviderWechat OAuthProvider = "wechat"
)

type SocialAccount struct {
	Id       int64
	UserId   int64
	Provider OAuthProvider
	OpenId   string
	UnionId  string
	Ctime    int64
}
