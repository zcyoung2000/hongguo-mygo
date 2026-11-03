package appdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppDirsAdoptLegacyData(t *testing.T) {
	home := t.TempDir()
	for _, base := range []string{appSupportBase, appCacheBase} {
		legacyDir := filepath.Join(home, libraryBasePath, base, legacyAppName)
		if err := os.MkdirAll(legacyDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacyDir, "data"), []byte("legacy"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	appDir, err := AppSupportDir(home)
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := AppCacheDir(home)

	for _, dir := range []string{appDir, cacheDir} {
		if got, err := os.ReadFile(filepath.Join(dir, "data")); err != nil || string(got) != "legacy" {
			t.Fatalf("legacy data was not adopted in %s: %q, %v", dir, got, err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), legacyAppName)); !os.IsNotExist(err) {
			t.Fatalf("legacy directory still exists beside %s: %v", dir, err)
		}
	}
}
