package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisFeedInbox_AddTrimsAndDedups(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	inbox := NewRedisFeedInbox(rdb, 2)
	userID := time.Now().UnixNano()
	key := FeedInboxKey(userID)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), key).Err() })

	if err := inbox.AddToUsers(ctx, []int64{userID}, 1); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if err := inbox.AddToUsers(ctx, []int64{userID}, 2); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if err := inbox.AddToUsers(ctx, []int64{userID}, 3); err != nil {
		t.Fatalf("add 3: %v", err)
	}
	n, err := rdb.ZCard(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want trimmed card=2, got %d", n)
	}
	ids, err := inbox.List(ctx, userID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 2 {
		t.Fatalf("want [3 2], got %v", ids)
	}

	if err = inbox.AddToUsers(ctx, []int64{userID}, 3); err != nil {
		t.Fatalf("duplicate add: %v", err)
	}
	n, err = rdb.ZCard(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("duplicate member must not grow inbox, card=%d", n)
	}
}
