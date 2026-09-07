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
	NewGORMFollowDAO,
	wire.Bind(new(FollowDAO), new(*GORMFollowDAO)),
	NewGORMNoteDAO,
	wire.Bind(new(NoteDAO), new(*GORMNoteDAO)),
	NewGORMLikeDAO,
	wire.Bind(new(LikeDAO), new(*GORMLikeDAO)),
	NewGORMCommentDAO,
	wire.Bind(new(CommentDAO), new(*GORMCommentDAO)),
	NewGORMFeedDAO,
	wire.Bind(new(FeedDAO), new(*GORMFeedDAO)),
)
