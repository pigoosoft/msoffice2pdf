package ofd

import "errors"

var (
	ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
	ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
	ErrInvalidPackage   = errors.New("ERR_OFD_INVALID_PACKAGE")
	ErrNoPages          = errors.New("ERR_OFD_NO_PAGES")
)
