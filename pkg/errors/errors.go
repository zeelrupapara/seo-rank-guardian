package errors

import "errors"

var (
	ErrBadRequest       = errors.New("bad request")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrInternalServer   = errors.New("internal server error")
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrUserNotFound     = errors.New("user not found")
	ErrUserAlreadyExist = errors.New("user already exists")
	ErrInvalidEmail     = errors.New("invalid email")
	ErrJobNotFound      = errors.New("job not found")
	ErrRunNotFound      = errors.New("run not found")
	ErrNoKeywords       = errors.New("at least one keyword is required")
	ErrNoRegions        = errors.New("at least one region is required")
	ErrJobNotActive     = errors.New("job is not active")
	ErrRunInProgress    = errors.New("a run is already in progress")
)
