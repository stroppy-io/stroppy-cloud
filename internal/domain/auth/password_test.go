package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword("secret123", hash) {
		t.Fatal("expected match")
	}
	if CheckPassword("wrong", hash) {
		t.Fatal("expected mismatch")
	}
}
