package native

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"myguo/internal/media"
)

func TestCachedArtworkReadsImageProxyCacheForOriginalAndProxyURLs(t *testing.T) {
	const original = "https://p9-novel.byteimg.com/test/cover.webp?x=1&y=2"
	dir := t.TempDir()
	path := filepath.Join(dir, "images", fmt.Sprintf("%x.jpg", md5.Sum([]byte(original))))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	want := []byte("image-proxy-cache-content")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	proxy := (&media.MediaServer{}).ImageURL(original)
	for _, url := range []string{original, proxy} {
		if got := cachedArtwork(dir, url); !bytes.Equal(got, want) {
			t.Fatalf("artwork lookup for %q = %q", url, got)
		}
	}
	for _, input := range [][2]string{{"", original}, {dir, ""}, {dir, "https://p9-novel.byteimg.com/missing"}} {
		if got := cachedArtwork(input[0], input[1]); len(got) != 0 {
			t.Fatal("missing artwork should remain empty")
		}
	}
}
