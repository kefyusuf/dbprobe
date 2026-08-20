package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type rule struct {
	root      string
	forbidden []string
}

func TestDependencyBoundaries(t *testing.T) {
	databaseDependencies := []string{
		"/adapters/",
		"database/sql",
		"go-sql-driver/mysql",
		"jackc/pgx",
		"mongo-driver",
		"gocql",
	}

	rules := []rule{
		{"../../internal/core", databaseDependencies},
		{"../../internal/app", databaseDependencies},
		{"../../internal/platform", databaseDependencies},
		{"../../internal/surfaces", databaseDependencies},
		{"../../sdk", append(append([]string{}, databaseDependencies...), "/internal/")},
	}

	for _, r := range rules {
		if _, err := os.Stat(r.root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				importPath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				for _, forbidden := range r.forbidden {
					if strings.Contains(importPath, forbidden) {
						t.Errorf("%s imports forbidden dependency %q", path, importPath)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
