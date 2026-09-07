package service

import "github.com/google/wire"

var ServiceSet = wire.NewSet(
	NewUserServiceImpl,
	wire.Bind(new(UserService), new(*UserServiceImpl)),
	NewSMSCodeService,
	wire.Bind(new(CodeService), new(*SMSCodeService)),
	NewSignInServiceImpl,
	wire.Bind(new(SignInService), new(*SignInServiceImpl)),
	NewRankServiceImpl,
	wire.Bind(new(RankService), new(*RankServiceImpl)),
	NewFollowServiceImpl,
	wire.Bind(new(FollowService), new(*FollowServiceImpl)),
	NewNoteServiceImpl,
	wire.Bind(new(NoteService), new(*NoteServiceImpl)),
	NewEngagementServiceImpl,
	wire.Bind(new(EngagementService), new(*EngagementServiceImpl)),
	NewFeedServiceImpl,
	wire.Bind(new(FeedService), new(*FeedServiceImpl)),
)
