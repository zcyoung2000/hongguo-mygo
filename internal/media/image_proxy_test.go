package media

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"myguo/internal/mediautil"
)

func TestImageProxyDownloadLimitRejectsOversizedResponse(t *testing.T) {
	payload := make([]byte, MaxImageDownloadBytes+1)
	payload[0], payload[1], payload[2] = 0xFF, 0xD8, 0xFF
	target := imageProxyTestServer(t, payload)
	response := httptest.NewRecorder()
	cacheDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0755); err != nil {
		t.Fatal(err)
	}
	ms := &MediaServer{cacheDir: cacheDir}
	ms.handleImage(response, httptest.NewRequest(http.MethodGet, "/img?u="+url.QueryEscape(target), nil))

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("response = %d/%s, want fallback", response.Code, response.Header().Get("Content-Type"))
	}
	cachePath := mediautil.ImageCachePath(cacheDir, target)
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("oversized image was cached: %v", err)
	}
}

func TestImageProxySipsFailureDoesNotCacheRawFile(t *testing.T) {
	raw := append([]byte("ftypheic"), make([]byte, 32)...)
	target := imageProxyTestServer(t, raw)
	response := httptest.NewRecorder()
	cacheDir := t.TempDir()
	ms := &MediaServer{cacheDir: cacheDir}
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0755); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/img?u="+url.QueryEscape(target), nil)
	t.Setenv("PATH", t.TempDir())
	ms.handleImage(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("response = %d/%s, want fallback", response.Code, response.Header().Get("Content-Type"))
	}
	cachePath := mediautil.ImageCachePath(cacheDir, target)
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("failed conversion was cached: %v", err)
	}
}

func TestImageProxyCachesNativeJPEG(t *testing.T) {
	payload := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 600)...)
	target := imageProxyTestServer(t, payload)
	first := httptest.NewRecorder()
	cacheDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cacheDir, "images"), 0755); err != nil {
		t.Fatal(err)
	}
	ms := &MediaServer{cacheDir: cacheDir}
	ms.handleImage(first, httptest.NewRequest(http.MethodGet, "/img?u="+url.QueryEscape(target), nil))

	if first.Code != http.StatusOK || first.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("first response = %d/%s, want cached JPEG", first.Code, first.Header().Get("Content-Type"))
	}
	cachePath := mediautil.ImageCachePath(cacheDir, target)
	if info, err := os.Stat(cachePath); err != nil || info.Size() != int64(len(payload)) {
		t.Fatalf("cache file = %v, %v", info, err)
	}
}

func imageProxyTestServer(t *testing.T, payload []byte) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverAddress := listener.Addr().String()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(payload)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	target := "http://test.byteimg.com/poster.jpg?signature=secret"
	oldClient := mediaHTTPClient
	mediaHTTPClient = &http.Client{
		Timeout:   time.Second,
		Transport: &rewriteTransport{serverURL: "http://" + serverAddress},
	}
	t.Cleanup(func() { mediaHTTPClient = oldClient })
	return target
}

type rewriteTransport struct {
	serverURL string
}

func (transport *rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	target, err := url.Parse(transport.serverURL + request.URL.Path + "?" + request.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	rewritten := request.Clone(request.Context())
	rewritten.URL = target
	rewritten.Host = target.Host
	return http.DefaultTransport.RoundTrip(rewritten)
}
