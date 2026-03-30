package dao

import "github.com/google/wire"

var DAOSet = wire.NewSet(
	NewGORMUserDAO,
	wire.Bind(new(UserDAO), new(*GORMUserDAO)),
)
