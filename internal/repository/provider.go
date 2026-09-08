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
	NewSignInRepositoryImpl,
	wire.Bind(new(SignInRepository), new(*SignInRepositoryImpl)),
	NewPointRepositoryImpl,
	wire.Bind(new(PointRepository), new(*PointRepositoryImpl)),
	NewRankRepositoryImpl,
	wire.Bind(new(RankRepository), new(*RankRepositoryImpl)),
	NewRedisActivityLogRepository,
	wire.Bind(new(ActivityLogRepository), new(*RedisActivityLogRepository)),
	NewEventOutboxRepositoryImpl,
	wire.Bind(new(EventOutboxRepository), new(*EventOutboxRepositoryImpl)),
	NewFollowRepositoryImpl,
	wire.Bind(new(FollowRepository), new(*FollowRepositoryImpl)),
	NewNoteRepositoryImpl,
	NewCachedNoteRepository,
	wire.Bind(new(NoteRepository), new(*CachedNoteRepository)),
	NewLikeRepositoryImpl,
	wire.Bind(new(LikeRepository), new(*LikeRepositoryImpl)),
	NewCommentRepositoryImpl,
	wire.Bind(new(CommentRepository), new(*CommentRepositoryImpl)),
	NewNotificationRepositoryImpl,
	wire.Bind(new(NotificationRepository), new(*NotificationRepositoryImpl)),
)
