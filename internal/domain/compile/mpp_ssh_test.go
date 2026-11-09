package compile

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestNewMPPKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		runA, runB   string
		wantSameKeys bool
	}{
		{"same run", "0f1d2c3b-aaaa-bbbb-cccc-111122223333", "0f1d2c3b-aaaa-bbbb-cccc-111122223333", true},
		{"another run", "0f1d2c3b-aaaa-bbbb-cccc-111122223333", "9e8d7c6b-aaaa-bbbb-cccc-111122223333", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := newMPPKey(tc.runA)
			if err != nil {
				t.Fatal(err)
			}
			b, err := newMPPKey(tc.runB)
			if err != nil {
				t.Fatal(err)
			}
			if same := a == b; same != tc.wantSameKeys {
				t.Errorf("keys equal = %v, want %v", same, tc.wantSameKeys)
			}
		})
	}
}

// The private key file is what ssh reads: it parses, its public half is the
// authorized_keys line, and the line has the form sshd expects.
func TestNewMPPKeyIsAnOpenSSHKey(t *testing.T) {
	t.Parallel()
	key, err := newMPPKey("0f1d2c3b-aaaa-bbbb-cccc-111122223333")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey([]byte(key.Private))
	if err != nil {
		t.Fatalf("the private key does not parse: %v", err)
	}
	if !strings.HasSuffix(key.Public, "\n") || strings.Count(key.Public, "\n") != 1 {
		t.Errorf("public key %q is not one line", key.Public)
	}
	authorized, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Public))
	if err != nil {
		t.Fatalf("the public key does not parse: %v", err)
	}
	if string(authorized.Marshal()) != string(signer.PublicKey().Marshal()) {
		t.Error("the public key is not the private key's")
	}
	if signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
		t.Errorf("key type %s", signer.PublicKey().Type())
	}
}

func BenchmarkNewMPPKey(b *testing.B) {
	for b.Loop() {
		if _, err := newMPPKey("0f1d2c3b-aaaa-bbbb-cccc-111122223333"); err != nil {
			b.Fatal(err)
		}
	}
}
