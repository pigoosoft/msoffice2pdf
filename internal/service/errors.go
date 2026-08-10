package service

import "errors"

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInvalidInput       = errors.New("invalid input")
	ErrFileMagic          = errors.New("ERR_FILE_MAGIC")
	ErrFileStructure      = errors.New("ERR_FILE_STRUCTURE")
	ErrExtEngineUnmapped  = errors.New("ERR_EXT_ENGINE_UNMAPPED")
)
