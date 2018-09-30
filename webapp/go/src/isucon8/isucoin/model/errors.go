package model

import "github.com/pkg/errors"

var (
	ErrBankUserNotFound   = errors.New("bank user not found")
	ErrBankUserConflict   = errors.New("bank user conflict")
	ErrUserNotFound       = errors.New("user not found")
	ErrCreditInsufficient = errors.New("銀行の残高が足りません")
	ErrParameterInvalid   = errors.New("parameter invalid")
)
