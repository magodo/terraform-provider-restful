package exparam

import (
	"net/url"
	"path/filepath"
	"regexp"
)

var (
	Pattern = regexp.MustCompile(`\$([\w\.]*)\(([\w.]+)\)`)
)

type FuncName string

const (
	FuncPathEscape    FuncName = "escape"
	FuncPathUnEscape  FuncName = "unescape"
	FuncQueryEscape   FuncName = "query_escape"
	FuncQueryUnEscape FuncName = "query_unescape"
	FuncBase          FuncName = "base"
	FuncURLPath       FuncName = "url_path"
	FuncTrimPath      FuncName = "trim_path"
)

type Func func(string) (string, error)

type FuncFactory struct {
	path string
}

func (f FuncFactory) Build() map[FuncName]Func {
	m := map[FuncName]Func{
		FuncPathEscape: func(s string) (string, error) {
			return url.PathEscape(s), nil
		},
		FuncPathUnEscape: url.PathUnescape,
		FuncQueryEscape: func(s string) (string, error) {
			return url.QueryEscape(s), nil
		},
		FuncQueryUnEscape: url.QueryUnescape,
		FuncBase: func(s string) (string, error) {
			return filepath.Base(s), nil
		},
		FuncURLPath: func(uRL string) (string, error) {
			u, err := url.Parse(uRL)
			if err != nil {
				return "", err
			}
			return u.Path, nil
		},
		FuncTrimPath: func(s string) (string, error) {
			return filepath.Rel(f.path, s)
		},
	}

	return m
}
