package worker

import (
	"errors"
	"fmt"
)

type permanentError struct {
	err error
}

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

func Permanent(err error) error {
	if err == nil || IsPermanent(err) {
		return err
	}
	return permanentError{err: err}
}

func Permanentf(format string, args ...any) error {
	return Permanent(fmt.Errorf(format, args...))
}

func IsPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
}
