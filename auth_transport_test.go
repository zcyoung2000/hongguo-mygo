package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestAuthTransportInjectsAccountCredentials(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `{"code":0,"data":[]}`)
	}))
	defer server.Close()

	transport := newAuthTransport()
	transport.setCredentials("sessionid=account", "token-value", "cn-gd")
	response, err := transport.Do(context.Background(), &sdk.Request{
		Method:  http.MethodGet,
		URL:     server.URL + "/reading/bookapi/bookmall/tab/v",
		Headers: map[string]string{"User-Agent": "agent"},
	})
	if err != nil {
		t.Fatalf("Do err = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got.Get("Cookie") != "sessionid=account" || got.Get("x-tt-token") != "token-value" || got.Get("x-tt-store-region") != "cn-gd" {
		t.Fatalf("credential headers = %v", got)
	}
}

func TestAuthTransportKeepsDeviceRegistrationAnonymous(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
	}))
	defer server.Close()

	transport := newAuthTransport()
	transport.setCredentials("sessionid=account", "token-value", "cn-gd")
	_, err := transport.Do(context.Background(), &sdk.Request{
		Method:  http.MethodPost,
		URL:     server.URL + "/service/2/device_register/",
		Headers: map[string]string{"Cookie": "guest"},
	})
	if err != nil {
		t.Fatalf("Do err = %v", err)
	}
	if got.Get("Cookie") != "guest" || got.Get("x-tt-token") != "" || got.Get("x-tt-store-region") != "" {
		t.Fatalf("registration headers = %v", got)
	}
	if strings.Contains(got.Get("Cookie"), "sessionid") {
		t.Fatalf("registration request used account cookie")
	}
}
