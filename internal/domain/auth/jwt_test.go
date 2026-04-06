package auth

import (
	"testing"
	"time"
)

func TestJWT_IssueAndParse(t *testing.T) {
	issuer := NewJWTIssuer("test-secret-key-32bytes-long!!")

	claims := Claims{
		UserID:   "u1",
		Username: "alice",
		TenantID: "t1",
		Role:     "operator",
		IsRoot:   false,
	}

	token, err := issuer.Issue(claims, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.UserID != "u1" || parsed.Username != "alice" || parsed.TenantID != "t1" {
		t.Fatalf("unexpected claims: %+v", parsed)
	}
	if parsed.Role != "operator" || parsed.IsRoot {
		t.Fatalf("unexpected role/root: %+v", parsed)
	}
}

func TestJWT_Expired(t *testing.T) {
	issuer := NewJWTIssuer("test-secret-key-32bytes-long!!")

	claims := Claims{UserID: "u1", Username: "alice"}
	token, _ := issuer.Issue(claims, -1*time.Minute)

	_, err := issuer.Parse(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	issuer1 := NewJWTIssuer("secret-one-32bytes-long-enough!")
	issuer2 := NewJWTIssuer("secret-two-32bytes-long-enough!")

	claims := Claims{UserID: "u1", Username: "alice"}
	token, _ := issuer1.Issue(claims, 15*time.Minute)

	_, err := issuer2.Parse(token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
