package redismocks

import (
	"context"
	"fmt"
	"reflect"

	"github.com/redis/go-redis/v9"
)

// MockCmdable 是一个最小可用的 redis.Cmdable mock。
// 它通过嵌入 redis.Cmdable 来满足接口，只显式实现当前测试会用到的 Eval。
// 其它方法如果被误调用，会因为底层嵌入字段为 nil 而立刻暴露问题。
type MockCmdable struct {
	redis.Cmdable
	expectations []*ExpectedEval
}

type ExpectedEval struct {
	script string
	keys   []string
	args   []interface{}
	val    int64
	err    error
}

func NewMockCmdable() *MockCmdable {
	return &MockCmdable{}
}

func (m *MockCmdable) ExpectEval(script string, keys []string, args ...interface{}) *ExpectedEval {
	exp := &ExpectedEval{
		script: script,
		keys:   append([]string(nil), keys...),
		args:   append([]interface{}(nil), args...),
	}
	m.expectations = append(m.expectations, exp)
	return exp
}

func (e *ExpectedEval) SetVal(val int64) *ExpectedEval {
	e.val = val
	return e
}

func (e *ExpectedEval) SetErr(err error) *ExpectedEval {
	e.err = err
	return e
}

func (m *MockCmdable) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	if len(m.expectations) == 0 {
		cmd.SetErr(fmt.Errorf("unexpected Eval call: script=%q keys=%v args=%v", script, keys, args))
		return cmd
	}

	exp := m.expectations[0]
	m.expectations = m.expectations[1:]

	if exp.script != script {
		cmd.SetErr(fmt.Errorf("unexpected script: want %q, got %q", exp.script, script))
		return cmd
	}
	if !reflect.DeepEqual(exp.keys, keys) {
		cmd.SetErr(fmt.Errorf("unexpected keys: want %v, got %v", exp.keys, keys))
		return cmd
	}
	if !reflect.DeepEqual(exp.args, args) {
		cmd.SetErr(fmt.Errorf("unexpected args: want %v, got %v", exp.args, args))
		return cmd
	}
	if exp.err != nil {
		cmd.SetErr(exp.err)
		return cmd
	}

	cmd.SetVal(exp.val)
	return cmd
}

func (m *MockCmdable) ExpectationsWereMet() error {
	if len(m.expectations) > 0 {
		return fmt.Errorf("there are %d unmet Eval expectations", len(m.expectations))
	}
	return nil
}
