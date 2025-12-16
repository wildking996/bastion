package core

import "errors"

var (
	ErrBastionNotFound = errors.New("bastion not found")
	ErrSSHConnection   = errors.New("ssh connection error")
	ErrResourceBusy    = errors.New("resource busy")
	ErrInvalidRequest  = errors.New("invalid request")
)

// BastionError is a custom error type
type BastionError struct {
	Message string
	Code    int
}

func (e *BastionError) Error() string {
	return e.Message
}

// NewBastionNotFoundError builds a 404 error
func NewBastionNotFoundError(msg string) *BastionError {
	return &BastionError{Message: msg, Code: 404}
}

// NewSSHConnectionError builds an SSH connection error
func NewSSHConnectionError(msg string) *BastionError {
	return &BastionError{Message: msg, Code: 502}
}

// NewResourceBusyError builds a resource-busy error
func NewResourceBusyError(msg string) *BastionError {
	return &BastionError{Message: msg, Code: 409}
}
