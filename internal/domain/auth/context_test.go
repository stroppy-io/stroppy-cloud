package auth

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	claims := &Claims{UserID: "u1", TenantID: "t1", Role: "operator"}
	ctx := WithClaims(context.Background(), claims)

	got := GetClaims(ctx)
	if got == nil || got.UserID != "u1" {
		t.Fatalf("expected u1, got %+v", got)
	}
	if TenantID(ctx) != "t1" {
		t.Fatalf("expected t1, got %s", TenantID(ctx))
	}
}

func TestRoleLevel(t *testing.T) {
	if RoleLevel("viewer") >= RoleLevel("operator") {
		t.Fatal("viewer should be less than operator")
	}
	if RoleLevel("operator") >= RoleLevel("owner") {
		t.Fatal("operator should be less than owner")
	}
}
