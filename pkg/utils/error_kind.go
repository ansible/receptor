package utils

import "fmt"

// ErrorWithKind represents an error wrapped with a designation of what kind of error it is.
type ErrorWithKind struct {
	err  error
	kind string
}

// Error returns the error text as a string.
func (ek ErrorWithKind) Error() string {
	return fmt.Sprintf("%s error: %s", ek.kind, ek.err)
}

// WrapErrorWithKind creates an ErrorWithKind that wraps an underlying error.
func WrapErrorWithKind(err error, kind string) ErrorWithKind {
	return ErrorWithKind{
		err:  err,
		kind: kind,
	}
}

// ErrorIsKind returns true if err is an ErrorWithKind of the specified kind, or false otherwise (including if nil).
func ErrorIsKind(err error, kind string) bool {
	ek, ok := err.(ErrorWithKind)
	if !ok {
		return false
	}
	return ek.kind == kind
}
