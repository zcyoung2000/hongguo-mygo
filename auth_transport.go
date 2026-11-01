package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
)

type authCredentialTransport interface {
	setCredentials(cookies, token, storeRegion string)
}

// authTransport sends the same official SDK requests as the public client,
// but lets an installed account credential cross the SDK boundary. The
// upstream SDK deliberately refuses credentials; Hongguo's player and user
// APIs have always sent them, so catalog reads use the same login state.
type authTransport struct {
	mu          sync.RWMutex
	client      *http.Client
	cookies     string
	token       string
	storeRegion string
}

func newAuthTransport() *authTransport {
	return &authTransport{client: &http.Client{Timeout: 30 * time.Second}}
}

func (t *authTransport) setCredentials(cookies, token, storeRegion string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cookies = strings.TrimSpace(cookies)
	t.token = strings.TrimSpace(token)
	t.storeRegion = strings.TrimSpace(storeRegion)
}

func (t *authTransport) Do(ctx context.Context, req *sdk.Request) (*sdk.Response, error) {
	if req == nil {
		return nil, errors.New("auth transport: nil request")
	}

	httpReq, err := http.NewRequestWithContext(ctx, methodOrGet(req.Method), req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	cookies, token, storeRegion := t.credentials()
	if cookies != "" && !isDeviceRegistration(req.URL) {
		httpReq.Header.Set("Cookie", cookies)
		httpReq.Header.Set("x-tt-store-region", storeRegion)
		if token != "" {
			httpReq.Header.Set("x-tt-token", token)
		}
	}

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	headers := make(map[string]string, len(resp.Header))
	for key, values := range resp.Header {
		if strings.EqualFold(key, "Set-Cookie") || strings.EqualFold(key, "Cookie") || len(values) == 0 {
			continue
		}
		headers[key] = values[0]
	}
	responseCookies := make(map[string]string, len(resp.Cookies()))
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "" {
			responseCookies[cookie.Name] = cookie.Value
		}
	}
	return &sdk.Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		Cookies:    responseCookies,
	}, nil
}

func (t *authTransport) credentials() (cookies, token, storeRegion string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cookies, t.token, t.storeRegion
}

func methodOrGet(method string) string {
	if strings.TrimSpace(method) == "" {
		return http.MethodGet
	}
	return method
}

func isDeviceRegistration(rawURL string) bool {
	return strings.Contains(rawURL, "/service/2/device_register/")
}
