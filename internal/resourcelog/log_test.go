package resourcelog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRedactURLRemovesQuery(t *testing.T) {
	input := "https://img.example.test/path/image.jpg?token=secret&user=browsing"
	if got := RedactURL(input); got != "https://img.example.test/path/image.jpg" {
		t.Fatalf("RedactURL() = %q", got)
	}
}

func TestRotatedLogKeepsLimitedGenerations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	for index := 3; index >= 1; index-- {
		if err := os.WriteFile(fmt.Sprintf("%s.%d", path, index), []byte("generation"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	file, err := OpenRotated(path, int64(len("current")), 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path + ".1", path + ".2", path + ".3"} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected generation %s: %v", name, err)
		}
	}
	if err := os.WriteFile(path, []byte("next"), 0644); err != nil {
		t.Fatal(err)
	}
	file, err = OpenRotated(path, int64(len("next")), 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".4"); !os.IsNotExist(err) {
		t.Fatalf("oldest generation was not removed: %v", err)
	}
	if _, err := os.Stat(path + ".3"); err != nil {
		t.Fatalf("expected newest generation: %v", err)
	}
}
