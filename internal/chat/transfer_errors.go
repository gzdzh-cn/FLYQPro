package chat

import (
	"errors"
	"syscall"
)

// TransferError is the structured error propagated through diagnostics and
// progress events. The underlying cause remains available for logs/tests but
// never needs to be shown directly to users.
type TransferError struct {
	Code      TransferErrorCode `json:"code"`
	Retryable bool              `json:"retryable"`
	Cause     error             `json:"-"`
}

func (e *TransferError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code)
}

func (e *TransferError) Unwrap() error { return e.Cause }

func newTransferError(code TransferErrorCode, retryable bool, cause error) error {
	return &TransferError{Code: code, Retryable: retryable, Cause: cause}
}

func transferErrorInfo(err error) (TransferErrorCode, bool) {
	var typed *TransferError
	if errors.As(err, &typed) && typed != nil {
		return typed.Code, typed.Retryable
	}
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return ErrInsufficientStorage, true
	}
	code := classifyTransferError(errorString(err))
	return code, code == ErrInsufficientStorage || code == ErrSessionNotReady
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
