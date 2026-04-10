package cache

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"user-center/internal/pkg/biztime"

	"github.com/redis/go-redis/v9"
)

type fakeRankCmdable struct {
	redis.Cmdable
	txPipeFactory       func() *fakeRankPipe
	zRevRangeWithScores func(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
	zRevRankFn          func(ctx context.Context, key, member string) *redis.IntCmd
	zScoreFn            func(ctx context.Context, key, member string) *redis.FloatCmd
}

func (f *fakeRankCmdable) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	pipe := f.txPipeFactory()
	return nil, fn(pipe)
}
func (f *fakeRankCmdable) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	return f.zRevRangeWithScores(ctx, key, start, stop)
}
func (f *fakeRankCmdable) ZRevRank(ctx context.Context, key, member string) *redis.IntCmd {
	return f.zRevRankFn(ctx, key, member)
}
func (f *fakeRankCmdable) ZScore(ctx context.Context, key, member string) *redis.FloatCmd {
	return f.zScoreFn(ctx, key, member)
}

type fakeRankPipe struct {
	redis.Pipeliner
	zAdds []struct {
		key    string
		member any
		score  float64
	}
	expireAts []struct {
		key string
		at  time.Time
	}
	zIncrBys []struct {
		key       string
		increment float64
		member    string
	}
}

func (p *fakeRankPipe) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	for _, m := range members {
		p.zAdds = append(p.zAdds, struct {
			key    string
			member any
			score  float64
		}{key: key, member: m.Member, score: m.Score})
	}
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(1)
	return cmd
}
func (p *fakeRankPipe) ExpireAt(ctx context.Context, key string, tm time.Time) *redis.BoolCmd {
	p.expireAts = append(p.expireAts, struct {
		key string
		at  time.Time
	}{key: key, at: tm})
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}
func (p *fakeRankPipe) ZIncrBy(ctx context.Context, key string, increment float64, member string) *redis.FloatCmd {
	p.zIncrBys = append(p.zIncrBys, struct {
		key       string
		increment float64
		member    string
	}{key: key, increment: increment, member: member})
	cmd := redis.NewFloatCmd(ctx)
	cmd.SetVal(increment)
	return cmd
}

func TestRedisRankCache_IncrSignInScore(t *testing.T) {
	t.Parallel()

	pipe := &fakeRankPipe{}
	c := NewRedisRankCache(&fakeRankCmdable{txPipeFactory: func() *fakeRankPipe { return pipe }})
	when := time.Date(2026, 4, 10, 9, 30, 0, 0, biztime.Location())

	if err := c.IncrSignInScore(context.Background(), 18, when, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipe.zAdds) != 1 || len(pipe.zIncrBys) != 1 || len(pipe.expireAts) != 2 {
		t.Fatalf("unexpected pipeline calls: zadd=%+v zincr=%+v expire=%+v", pipe.zAdds, pipe.zIncrBys, pipe.expireAts)
	}
	if pipe.zAdds[0].key != "rank:active:daily:20260410" || pipe.zAdds[0].member != "18" {
		t.Fatalf("unexpected daily zadd: %+v", pipe.zAdds[0])
	}
	if pipe.zIncrBys[0].key != "rank:active:monthly:202604" || pipe.zIncrBys[0].member != "18" || pipe.zIncrBys[0].increment != 5 {
		t.Fatalf("unexpected monthly zincrby: %+v", pipe.zIncrBys[0])
	}
	wantDailyExpire := biztime.NextDayStart(when).Add(7 * 24 * time.Hour)
	wantMonthExpire := biztime.NextMonthStart(when).Add(365 * 24 * time.Hour)
	if !pipe.expireAts[0].at.Equal(wantDailyExpire) || !pipe.expireAts[1].at.Equal(wantMonthExpire) {
		t.Fatalf("unexpected expire at: %+v", pipe.expireAts)
	}
}

func TestRedisRankCache_TopNDaily(t *testing.T) {
	t.Parallel()

	c := NewRedisRankCache(&fakeRankCmdable{zRevRangeWithScores: func(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
		if key != "rank:active:daily:20260410" || start != 0 || stop != 2 {
			t.Fatalf("unexpected args: key=%s start=%d stop=%d", key, start, stop)
		}
		cmd := redis.NewZSliceCmd(ctx)
		cmd.SetVal([]redis.Z{{Member: "10", Score: 11}, {Member: "bad"}, {Member: 22, Score: 9}, {Member: "20", Score: 8}})
		return cmd
	}})

	items, err := c.TopNDaily(context.Background(), time.Date(2026, 4, 10, 11, 0, 0, 0, biztime.Location()), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(items, []RankUserScore{{UserID: 10, Score: 11}, {UserID: 20, Score: 8}}) {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestRedisRankCache_GetDailyRank(t *testing.T) {
	t.Parallel()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		c := NewRedisRankCache(&fakeRankCmdable{
			zRevRankFn: func(ctx context.Context, key, member string) *redis.IntCmd {
				cmd := redis.NewIntCmd(ctx)
				cmd.SetVal(2)
				return cmd
			},
			zScoreFn: func(ctx context.Context, key, member string) *redis.FloatCmd {
				cmd := redis.NewFloatCmd(ctx)
				cmd.SetVal(15)
				return cmd
			},
		})
		rank, score, found, err := c.GetDailyRank(context.Background(), 9, time.Date(2026, 4, 10, 0, 0, 0, 0, biztime.Location()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || rank != 3 || score != 15 {
			t.Fatalf("unexpected result: rank=%d score=%v found=%v", rank, score, found)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		c := NewRedisRankCache(&fakeRankCmdable{
			zRevRankFn: func(ctx context.Context, key, member string) *redis.IntCmd {
				cmd := redis.NewIntCmd(ctx)
				cmd.SetErr(redis.Nil)
				return cmd
			},
			zScoreFn: func(ctx context.Context, key, member string) *redis.FloatCmd {
				t.Fatal("zscore should not be called when rank misses")
				return nil
			},
		})
		_, _, found, err := c.GetDailyRank(context.Background(), 9, time.Date(2026, 4, 10, 0, 0, 0, 0, biztime.Location()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("want found=false")
		}
	})

	t.Run("zscore error bubbles up", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("redis down")
		c := NewRedisRankCache(&fakeRankCmdable{
			zRevRankFn: func(ctx context.Context, key, member string) *redis.IntCmd {
				cmd := redis.NewIntCmd(ctx)
				cmd.SetVal(0)
				return cmd
			},
			zScoreFn: func(ctx context.Context, key, member string) *redis.FloatCmd {
				cmd := redis.NewFloatCmd(ctx)
				cmd.SetErr(wantErr)
				return cmd
			},
		})
		_, _, _, err := c.GetDailyRank(context.Background(), 9, time.Date(2026, 4, 10, 0, 0, 0, 0, biztime.Location()))
		if !errors.Is(err, wantErr) {
			t.Fatalf("want err %v, got %v", wantErr, err)
		}
	})
}
