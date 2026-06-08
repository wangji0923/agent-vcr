package visualize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisualizePackageDoesNotImportAdapters(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry == nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		needleSlash := strings.Join([]string{"internal", "adapters"}, "/")
		needleBackslash := strings.Join([]string{"internal", "adapters"}, "\\")
		content := string(data)
		if strings.Contains(content, needleSlash) || strings.Contains(content, needleBackslash) {
			t.Fatalf("%s must not import or reference internal adapters", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk visualize package: %v", err)
	}
}
