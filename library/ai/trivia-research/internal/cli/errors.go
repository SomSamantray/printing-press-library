package cli

import "fmt"

func usageErr(err error) error  { return &cliError{code: 2, err: fmt.Errorf("usage: %w", err)} }
func notFoundErr(err error) error { return &cliError{code: 3, err: fmt.Errorf("not found: %w", err)} }
func authErr(err error) error { return &cliError{code: 4, err: fmt.Errorf("auth: %w", err)} }
func apiErr(err error) error { return &cliError{code: 5, err: fmt.Errorf("api: %w", err)} }
func rateLimitErr(err error) error { return &cliError{code: 7, err: fmt.Errorf("rate limit: %w", err)} }

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }
