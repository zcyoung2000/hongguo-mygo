package uiapp

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestChineseReadmeAndSettingsParityDocsStayCurrent(t *testing.T) {
	readme, err := os.ReadFile("../../README.zh-CN.md")
	if err != nil {
		t.Fatalf("read Chinese README: %v", err)
	}
	nodeTests := regexp.MustCompile(`(?m)^node\s+(\S+\.test\.(?:js|cjs))\s*$`)
	matches := nodeTests.FindAllStringSubmatch(string(readme), -1)
	if len(matches) == 0 {
		t.Fatal("Chinese README does not list a Node test script")
	}
	for _, match := range matches {
		if _, err := os.Stat("../../" + match[1]); err != nil {
			t.Fatalf("Chinese README references missing test script %s: %v", match[1], err)
		}
	}

	parity, err := os.ReadFile("../../docs/native-feature-parity.md")
	if err != nil {
		t.Fatalf("read native feature parity document: %v", err)
	}
	settingsRow := regexp.MustCompile(`(?m)^\| Settings \|.*\|\s*$`).FindString(string(parity))
	if settingsRow == "" {
		t.Fatal("native feature parity document has no Settings row")
	}
	if !strings.Contains(settingsRow, "renderSettings") {
		t.Fatal("Settings parity row does not identify the native renderer")
	}
	if regexp.MustCompile(`(?i)\babout\b\s*\|`).MatchString(settingsRow) {
		t.Fatal("Settings parity row lists the removed about feature")
	}
}
