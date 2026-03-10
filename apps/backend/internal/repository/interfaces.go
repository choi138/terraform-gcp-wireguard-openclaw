package repository

import "errors"

// ErrNotFound is returned when a requested row/resource does not exist.
var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
)
