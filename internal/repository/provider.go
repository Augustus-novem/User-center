package repository

import (
	"github.com/google/wire"
)

var RepoSet = wire.NewSet(
	NewCachedCodeRepository,
	wire.Bind(new(CodeRepository), new(*CachedCodeRepository)),
	NewCachedUserRepository,
	wire.Bind(new(UserRepository), new(*CachedUserRepository)),
	NewSocialAccountRepositoryImpl,
	wire.Bind(new(SocialAccountRepository), new(*SocialAccountRepositoryImpl)),
)
