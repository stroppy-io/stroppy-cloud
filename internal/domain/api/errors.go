package api

import "errors"

var (
	errInvalidCredentials  = errors.New("invalid credentials")
	errInvalidRefreshToken = errors.New("invalid or expired refresh token")
	errMissingToken        = errors.New("missing authorization token")
)
