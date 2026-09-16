package cloud

import (
	"context"
	"errors"
)

// IAMToken resolves a short-lived token in the calling activity. Neither the
// service-account key nor the token is returned through workflow history.
func (y Yandex) IAMToken(ctx context.Context, credentials string) (token string, err error) {
	sdk, err := y.sdk(ctx, credentials)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, sdk.Shutdown(ctx)) }()
	response, err := sdk.CreateIAMToken(ctx)
	if err != nil {
		return "", err
	}
	return response.IamToken, nil
}
