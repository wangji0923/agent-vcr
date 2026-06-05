package analysis

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalysisAndCheckDoNotImportAdapters(t *testing.T) {
	for _, dir := range []string{".", "../check"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, imp := range file.Imports {
				value := strings.Trim(imp.Path.Value, `"`)
				if strings.Contains(value, "/internal/adapters") {
					t.Fatalf("%s imports forbidden adapter package %s", path, value)
				}
			}
		}
	}
}
