package behavior

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBehaviorPackageDoesNotImportAdapters(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read behavior package dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		needleSlash := strings.Join([]string{"internal", "adapters"}, "/")
		needleBackslash := strings.Join([]string{"internal", "adapters"}, "\\")
		content := string(data)
		if strings.Contains(content, needleSlash) || strings.Contains(content, needleBackslash) {
			t.Fatalf("%s must not import or reference internal adapters", path)
		}
	}
}
