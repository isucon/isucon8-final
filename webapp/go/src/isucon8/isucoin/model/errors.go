package model

import "github.com/pkg/errors"

var (
	ErrBankUserNotFound = errors.New("bank user not found")
	ErrBankUserConflict = errors.New("bank user conflict")
	ErrUserNotFound     = errors.New("user not found")
)
