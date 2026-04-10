package service

import "github.com/google/wire"

var ServiceSet = wire.NewSet(
	NewUserServiceImpl,
	wire.Bind(new(UserService), new(*UserServiceImpl)),
	NewSMSCodeService,
	wire.Bind(new(CodeService), new(*SMSCodeService)),
	NewSignInService,
	wire.Bind(new(SignInService), new(*SignInServiceImpl)),
	NewRankServiceImpl,
	wire.Bind(new(RankService), new(*RankServiceImpl)),
)
