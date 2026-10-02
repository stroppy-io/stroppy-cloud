//go:build integration

package application

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// jwtSigner stands in for the Kratos tokenizer: ES256 key, the public half
// written to a JWKS file the verifier reads. Tokens carry the claims of
// deployments/kratos/claims.jsonnet.
type jwtSigner struct {
	key      jwk.Key
	jwksPath string
}

func newJWTSigner(t *testing.T) *jwtSigner {
	t.Helper()
	raw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa: %v", err)
	}
	key, err := jwk.Import(raw)
	if err != nil {
		t.Fatalf("jwk: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, "test-"+uuid.NewString()[:8]); err != nil {
		t.Fatalf("kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, "ES256"); err != nil {
		t.Fatalf("alg: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(key); err != nil {
		t.Fatalf("set: %v", err)
	}
	pub, err := jwk.PublicKeyOf(key)
	if err != nil {
		t.Fatalf("public: %v", err)
	}
	pubSet := jwk.NewSet()
	if err := pubSet.AddKey(pub); err != nil {
		t.Fatalf("pub set: %v", err)
	}
	rawSet, err := json.Marshal(pubSet)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	path := filepath.Join(t.TempDir(), "test-jwks.json")
	if err := os.WriteFile(path, rawSet, 0o600); err != nil {
		t.Fatalf("write jwks: %v", err)
	}
	return &jwtSigner{key: key, jwksPath: path}
}

// token signs a session JWT for the identity with the given claims.
func (s *jwtSigner) token(t *testing.T, sub, email string, verified bool, name string) string {
	t.Helper()
	tok := jwt.New()
	_ = tok.Set(jwt.IssuerKey, "stroppy-cloud")                    //nolint:errcheck // fixed claims
	_ = tok.Set(jwt.AudienceKey, []string{"stroppy-cloud"})        //nolint:errcheck
	_ = tok.Set(jwt.SubjectKey, sub)                               //nolint:errcheck
	_ = tok.Set("sid", "session-"+sub[:8])                         //nolint:errcheck
	_ = tok.Set("email", email)                                    //nolint:errcheck
	_ = tok.Set("email_verified", verified)                        //nolint:errcheck
	_ = tok.Set("name", name)                                      //nolint:errcheck
	_ = tok.Set(jwt.ExpirationKey, time.Now().Add(10*time.Minute)) //nolint:errcheck
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.ES256(), s.key))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return string(signed)
}
