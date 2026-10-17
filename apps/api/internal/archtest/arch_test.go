package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/belchiorjr/game-forge/apps/api"

// importsUnder returns, for each non-test .go file under dir, its import paths.
func importsUnder(t *testing.T, dir string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			out[path] = append(out[path], strings.Trim(imp.Path.Value, "\""))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func isStdlib(p string) bool {
	first := strings.SplitN(p, "/", 2)[0]
	return !strings.Contains(first, ".")
}

func TestDomainDependsOnNothing(t *testing.T) {
	for file, imps := range importsUnder(t, "../domain") {
		for _, p := range imps {
			if !isStdlib(p) {
				t.Errorf("%s imports %s; domain may only use the standard library", file, p)
			}
		}
	}
}

func TestUsecaseDependsOnDomainAndPorts(t *testing.T) {
	for file, imps := range importsUnder(t, "../usecase") {
		for _, p := range imps {
			switch {
			case isStdlib(p):
			case p == module+"/internal/domain", p == module+"/internal/usecase/port":
			default:
				t.Errorf("%s imports %s; use cases may only import the standard library, domain and usecase/port", file, p)
			}
		}
	}
}

func TestAdaptersDoNotImportEachOther(t *testing.T) {
	adapters, err := filepath.Glob("../adapter/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range adapters {
		name := filepath.Base(dir)
		for file, imps := range importsUnder(t, dir) {
			for _, p := range imps {
				if strings.HasPrefix(p, module+"/internal/adapter/") && !strings.HasPrefix(p, module+"/internal/adapter/"+name) {
					t.Errorf("%s imports %s; adapters must not depend on each other", file, p)
				}
			}
		}
	}
}
