package courier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestEveryTemplateIsServed(t *testing.T) {
	for _, tpl := range []string{"verification_code", "recovery_code"} {
		for _, state := range []string{"valid", "invalid"} {
			for _, file := range Files {
				rec := get(t, Prefix+"/v0.6.0/"+tpl+"/"+state+"/"+file)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
				require.NotEmpty(t, rec.Body.String())
			}
		}
	}
}

func TestUnknownTemplateIsNotFound(t *testing.T) {
	for _, path := range []string{
		Prefix + "/v1/login_code/valid/email.body.gotmpl",
		Prefix + "/v1/verification_code/stale/email.body.gotmpl",
		Prefix + "/v1/verification_code/valid/other.txt",
		Prefix + "/v1/verification_code/valid",
		Prefix + "/v1/..%2Fcourier.go/valid/email.body.gotmpl",
		"/internal/other",
	} {
		require.Equal(t, http.StatusNotFound, get(t, path).Code, path)
	}
}
