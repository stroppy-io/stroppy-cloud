package cloud

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// IAMCredential is activity-local secret material, never a workflow payload.
type IAMCredential struct {
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"-"`
}

func (IAMCredential) String() string { return "[redacted IAM credential]" }

// IAMToken resolves a short-lived token in the calling activity. Neither the
// service-account key nor the token is returned through workflow history.
func (y Yandex) IAMToken(ctx context.Context, credentials string) (token string, err error) {
	issued, err := y.IAMTokenWithExpiration(ctx, credentials)
	return issued.Token, err
}

func (y Yandex) IAMTokenWithExpiration(ctx context.Context, credentials string) (issued IAMCredential, err error) {
	sdk, err := y.sdk(ctx, credentials)
	if err != nil {
		return issued, err
	}
	defer func() { err = errors.Join(err, sdk.Shutdown(ctx)) }()
	response, err := sdk.CreateIAMToken(ctx)
	if err != nil {
		return issued, err
	}
	if response.ExpiresAt == nil || response.IamToken == "" {
		return issued, fmt.Errorf("IAM response has no token or expiry")
	}
	return IAMCredential{Token: response.IamToken, ExpiresAt: response.ExpiresAt.AsTime()}, nil
}
