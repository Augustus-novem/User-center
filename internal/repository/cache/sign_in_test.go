package cache

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeSignInCmdable struct {
	redis.Cmdable
	txPipeFactory func() *fakeSignInPipe
	getBitFn      func(ctx context.Context, key string, offset int64) *redis.IntCmd
	existsFn      func(ctx context.Context, keys ...string) *redis.IntCmd
	pipelineFn    func() redis.Pipeliner
}

func (f *fakeSignInCmdable) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	pipe := f.txPipeFactory()
	return nil, fn(pipe)
}
func (f *fakeSignInCmdable) GetBit(ctx context.Context, key string, offset int64) *redis.IntCmd {
	return f.getBitFn(ctx, key, offset)
}
func (f *fakeSignInCmdable) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return f.existsFn(ctx, keys...)
}
func (f *fakeSignInCmdable) Pipeline() redis.Pipeliner { return f.pipelineFn() }

type fakeSignInPipe struct {
	redis.Pipeliner
	setBits []struct {
		key    string
		offset int64
		value  int
	}
	expireKeys []struct {
		key string
		ttl time.Duration
	}
	getBitVals map[int64]int64
	execErr    error
}

func (p *fakeSignInPipe) SetBit(ctx context.Context, key string, offset int64, value int) *redis.IntCmd {
	p.setBits = append(p.setBits, struct {
		key    string
		offset int64
		value  int
	}{key: key, offset: offset, value: value})
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(0)
	return cmd
}
func (p *fakeSignInPipe) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	p.expireKeys = append(p.expireKeys, struct {
		key string
		ttl time.Duration
	}{key: key, ttl: expiration})
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}
func (p *fakeSignInPipe) GetBit(ctx context.Context, key string, offset int64) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(p.getBitVals[offset])
	return cmd
}
func (p *fakeSignInPipe) Exec(ctx context.Context) ([]redis.Cmder, error) { return nil, p.execErr }

func TestRedisSignInCache_SetSigned(t *testing.T) {
	t.Parallel()

	pipe := &fakeSignInPipe{}
	c := NewRedisSignInCache(&fakeSignInCmdable{txPipeFactory: func() *fakeSignInPipe { return pipe }})
	signDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	if err := c.SetSigned(context.Background(), 12, signDate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipe.setBits) != 1 {
		t.Fatalf("want 1 setbit call, got %d", len(pipe.setBits))
	}
	got := pipe.setBits[0]
	if got.key != "sign:12:2026:04" || got.offset != 9 || got.value != 1 {
		t.Fatalf("unexpected setbit call: %+v", got)
	}
	if len(pipe.expireKeys) != 1 || pipe.expireKeys[0].ttl != 180*24*time.Hour {
		t.Fatalf("unexpected expire calls: %+v", pipe.expireKeys)
	}
}

func TestRedisSignInCache_IsSignedOnDate(t *testing.T) {
	t.Parallel()

	var seenKey string
	var seenOffset int64
	c := NewRedisSignInCache(&fakeSignInCmdable{getBitFn: func(ctx context.Context, key string, offset int64) *redis.IntCmd {
		seenKey = key
		seenOffset = offset
		cmd := redis.NewIntCmd(ctx)
		cmd.SetVal(1)
		return cmd
	}})

	signed, err := c.IsSignedOnDate(context.Background(), 12, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !signed || seenKey != "sign:12:2026:04" || seenOffset != 9 {
		t.Fatalf("unexpected result: signed=%v key=%s offset=%d", signed, seenKey, seenOffset)
	}
}

func TestRedisSignInCache_GetMonthSignedDays(t *testing.T) {
	t.Parallel()

	t.Run("cache miss", func(t *testing.T) {
		t.Parallel()
		c := NewRedisSignInCache(&fakeSignInCmdable{
			existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
				cmd := redis.NewIntCmd(ctx)
				cmd.SetVal(0)
				return cmd
			},
		})
		days, found, err := c.GetMonthSignedDays(context.Background(), 3, 2026, time.April)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found || days != nil {
			t.Fatalf("want cache miss, got found=%v days=%v", found, days)
		}
	})

	t.Run("cache hit returns bitset days", func(t *testing.T) {
		t.Parallel()
		pipe := &fakeSignInPipe{getBitVals: map[int64]int64{0: 1, 2: 1, 29: 1}}
		c := NewRedisSignInCache(&fakeSignInCmdable{
			existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
				cmd := redis.NewIntCmd(ctx)
				cmd.SetVal(1)
				return cmd
			},
			pipelineFn: func() redis.Pipeliner { return pipe },
		})
		days, found, err := c.GetMonthSignedDays(context.Background(), 3, 2024, time.February)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("want found=true")
		}
		if !reflect.DeepEqual(days, []int{1, 3}) {
			t.Fatalf("unexpected days: %v", days)
		}
	})
}

func TestRedisSignInCache_BatchSetMonthSignedDays(t *testing.T) {
	t.Parallel()

	pipe := &fakeSignInPipe{}
	c := NewRedisSignInCache(&fakeSignInCmdable{txPipeFactory: func() *fakeSignInPipe { return pipe }})
	if err := c.BatchSetMonthSignedDays(context.Background(), 9, 2026, time.April, []int{2, 5, 30}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantOffsets := []int64{1, 4, 29}
	if len(pipe.setBits) != len(wantOffsets) {
		t.Fatalf("unexpected setbit calls: %+v", pipe.setBits)
	}
	for i, off := range wantOffsets {
		if pipe.setBits[i].key != "sign:9:2026:04" || pipe.setBits[i].offset != off || pipe.setBits[i].value != 1 {
			t.Fatalf("unexpected setbit[%d]: %+v", i, pipe.setBits[i])
		}
	}
	if len(pipe.expireKeys) != 1 {
		t.Fatalf("unexpected expire calls: %+v", pipe.expireKeys)
	}
}

func TestRedisSignInCache_GetMonthSignedDays_ExecError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("pipeline failed")
	pipe := &fakeSignInPipe{execErr: wantErr}
	c := NewRedisSignInCache(&fakeSignInCmdable{
		existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx)
			cmd.SetVal(1)
			return cmd
		},
		pipelineFn: func() redis.Pipeliner { return pipe },
	})

	_, _, err := c.GetMonthSignedDays(context.Background(), 3, 2026, time.April)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want err %v, got %v", wantErr, err)
	}
}
