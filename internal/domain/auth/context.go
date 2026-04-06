package auth

import (
	"context"
)

type ctxKey int

const claimsKey ctxKey = iota

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func TenantID(ctx context.Context) string {
	if c := GetClaims(ctx); c != nil {
		return c.TenantID
	}
	return ""
}

func RoleLevel(role string) int {
	switch role {
	case "viewer":
		return 1
	case "operator":
		return 2
	case "owner":
		return 3
	default:
		return 0
	}
}
