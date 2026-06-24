package provider

import "errors"

var (
	ErrNotFound      = errors.New("session not found")
	ErrNotInstalled  = errors.New("provider not installed")
	ErrAlreadyExists = errors.New("session already migrated")
	ErrEmptySession  = errors.New("session has no messages")
)
