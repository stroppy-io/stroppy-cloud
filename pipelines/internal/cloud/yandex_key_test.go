package cloud

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYandexServiceAccountKeyJSON(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]string{
		"id": "test-key", "service_account_id": "test-sa",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
	})
	require.NoError(t, err)
	wrapped, err := json.Marshal(yandexCredentials{SAKeyJSON: string(raw)})
	require.NoError(t, err)
	got, err := YandexServiceAccountKeyJSON(string(wrapped))
	require.NoError(t, err)
	require.Equal(t, string(raw), got)
	for _, invalid := range []string{"", `{}`, `{"sa_key_json":"secret-malformed-json"}`, `{"sa_key_json":"{\"id\":\"x\",\"service_account_id\":\"sa\",\"private_key\":\"secret-invalid-key\"}"}`} {
		got, err := YandexServiceAccountKeyJSON(invalid)
		require.Error(t, err)
		require.Empty(t, got)
		require.NotContains(t, err.Error(), "secret-")
	}
}
