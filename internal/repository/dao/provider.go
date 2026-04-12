package dao

import "github.com/google/wire"

var DAOSet = wire.NewSet(
	NewGORMUserDAO,
	wire.Bind(new(UserDAO), new(*GORMUserDAO)),
	NewGormSocialAccountDAO,
	wire.Bind(new(SocialAccountDAO), new(*GORMSocialAccountDAO)),
	NewGORMSignInDAO,
	wire.Bind(new(SignInDAO), new(*GORMSignInDAO)),
	NewGORMPointDAO,
	wire.Bind(new(PointDAO), new(*GORMPointDAO)),
	NewGORMEventOutboxDAO,
	wire.Bind(new(EventOutboxDAO), new(*GORMEventOutboxDAO)),
)
