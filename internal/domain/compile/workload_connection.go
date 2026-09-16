package compile

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
)

// connectionURL applies protocol options to the DSN, not to DriverRunConfig,
// which rejects sslmode, charset, tls and applicationName as unknown fields.
// Split only the query so unresolved ${ip:role:...} hosts remain valid.
func connectionURL(dsn string, protocol catalog.Protocol, conn map[string]any) (string, error) {
	var keys []string
	switch protocol { //nolint:exhaustive // other protocols have no URL options in connection
	case catalog.ProtoPg, catalog.ProtoCockroach:
		keys = []string{"sslmode", "application_name"}
	case catalog.ProtoMySQL:
		keys = []string{"tls", "charset"}
	default:
		return dsn, nil
	}
	prefix, suffix := "", dsn
	if protocol == catalog.ProtoMySQL {
		// go-sql-driver/mysql permits literal question marks in passwords.
		if slash := strings.LastIndex(dsn, "/"); slash >= 0 {
			prefix, suffix = dsn[:slash+1], dsn[slash+1:]
		}
	}
	base, query, _ := strings.Cut(suffix, "?")
	base = prefix + base
	values, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("invalid connection query options")
	}
	changed := false
	for _, key := range keys {
		if value, ok := conn[key].(string); ok && value != "" {
			values.Set(key, value)
			changed = true
		}
	}
	if !changed {
		return dsn, nil
	}
	return base + "?" + values.Encode(), nil
}
