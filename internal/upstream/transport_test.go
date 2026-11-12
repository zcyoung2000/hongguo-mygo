package upstream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type recordingTransport struct {
	requests  []*http.Request
	bodies    []string
	responses []*http.Response
	errs      []error
}

func (transport *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	index := len(transport.requests)
	transport.requests = append(transport.requests, request.Clone(request.Context()))
	body := []byte(nil)
	if request.Body != nil {
		read, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		body = read
	}
	transport.bodies = append(transport.bodies, string(body))
	if index >= len(transport.responses) {
		return nil, errors.New("unexpected transport call")
	}
	response := transport.responses[index]
	err := error(nil)
	if index < len(transport.errs) {
		err = transport.errs[index]
	}
	if response != nil {
		response.Request = request
	}
	return response, err
}

func TestTransportRetriesNetworkFailureAndReusesBody(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		errs: []error{errors.New("connection reset")},
		responses: []*http.Response{
			nil,
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":0}`)), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, err := http.NewRequest(http.MethodPost, "https://"+DefaultPlaybackHosts[0]+"/novel/player/video_model/v1/", strings.NewReader(`{"video_id":1}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if len(next.requests) != 2 || next.requests[0].URL.Host != DefaultPlaybackHosts[0] || next.requests[1].URL.Host != DefaultPlaybackHosts[1] {
		t.Fatalf("hosts = %v", requestHosts(next.requests))
	}
	if next.bodies[0] != next.bodies[1] || next.bodies[0] != `{"video_id":1}` {
		t.Fatalf("bodies = %q", next.bodies)
	}
}

func TestTransportBusinessErrorDoesNotSwitchHost(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(`{"code":400}`)), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, _ := http.NewRequest(http.MethodPost, "https://"+DefaultCommentHosts[0]+"/novel/commentapi/comment/add/v1/", strings.NewReader(`{}`))
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || len(next.requests) != 1 {
		t.Fatalf("status = %d calls = %d", response.StatusCode, len(next.requests))
	}
}

func TestTransportRetriesFiveHundredAndEmptyResponses(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("busy")), Header: http.Header{}},
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}},
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "ok" || len(next.requests) != 3 {
		t.Fatalf("body = %q calls = %d", body, len(next.requests))
	}
}

func TestTransportReturnsLastErrorAfterAttemptLimit(t *testing.T) {
	picker := NewHostPicker(nil)
	cause := errors.New("timed out")
	next := &recordingTransport{
		responses: []*http.Response{nil, nil, nil},
		errs:      []error{cause, cause, cause, cause},
	}
	transport := NewTransport(picker, next)
	transport.SetMaxAttempts(3)
	request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
	if _, err := transport.RoundTrip(request); !errors.Is(err, cause) {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if len(next.requests) != 3 {
		t.Fatalf("calls = %d", len(next.requests))
	}
}

func TestTransportDisabledPreservesSingleOrigin(t *testing.T) {
	picker := NewHostPicker(nil)
	picker.MarkBad(FamilyPlayback, DefaultPlaybackHosts[0])
	next := &recordingTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	transport.SetEnabled(false)
	request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()
	if len(next.requests) != 1 || next.requests[0].URL.Host != DefaultPlaybackHosts[0] {
		t.Fatalf("requests = %v", requestHosts(next.requests))
	}
}

func TestTransportMarksSuccessfulHostGoodAndFailedHostBad(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		errs: []error{errors.New("connection refused")},
		responses: []*http.Response{
			nil,
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	failed := findState(t, picker.States(), FamilyPlayback, DefaultPlaybackHosts[0])
	good := findState(t, picker.States(), FamilyPlayback, DefaultPlaybackHosts[1])
	if failed.Health != -DefaultFailurePenalty || failed.ConsecutiveFailures != 1 {
		t.Fatalf("failed host state = %+v", failed)
	}
	if good.Health != DefaultSuccessGain || good.ConsecutiveFailures != 0 {
		t.Fatalf("good host state = %+v", good)
	}
}

func TestTransportLeavesUnmanagedHostAlone(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, _ := http.NewRequest(http.MethodGet, "https://example.test/reading/bookapi/search/tab/v", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()
	if len(next.requests) != 1 || next.requests[0].URL.Host != "example.test" {
		t.Fatalf("requests = %v", requestHosts(next.requests))
	}
}

func TestHostOf(t *testing.T) {
	if got := HostOf("https://example.test/path?query=1"); got != "example.test" {
		t.Fatalf("HostOf() = %q", got)
	}
}

func TestTransportResponseBodyPrefixIsReadable(t *testing.T) {
	picker := NewHostPicker(nil)
	next := &recordingTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(io.MultiReader(bytes.NewReader([]byte("a")), strings.NewReader("bc"))), Header: http.Header{}},
		},
	}
	transport := NewTransport(picker, next)
	request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "abc" {
		t.Fatalf("body = %q", body)
	}
}

func requestHosts(requests []*http.Request) []string {
	values := make([]string, len(requests))
	for index, request := range requests {
		values[index] = request.URL.Host
	}
	return values
}

type localHTTP2Transport struct {
	next   http.RoundTripper
	target *url.URL
}

func (r localHTTP2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = r.target.Scheme
	copy.URL.Host = r.target.Host
	return r.next.RoundTrip(copy)
}
func TestTransportKeepsRealHTTP2ResponseOpen(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			const payload = `{"code":0,"data":{"video_list":[{"id":"episode"}]}}`
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				// Flush the prefix so net/http cannot buffer the entire body before Close.
				_, _ = io.WriteString(w, payload[:1])
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, payload[1:])
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			target, _ := url.Parse(server.URL)
			transport := NewTransport(NewHostPicker(nil), localHTTP2Transport{server.Client().Transport, target})
			transport.SetMaxAttempts(1)
			request, _ := http.NewRequest(http.MethodGet, "https://"+DefaultPlaybackHosts[0]+"/reading/bookapi/search/tab/v", nil)
			response, err := transport.RoundTrip(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.ProtoMajor != 2 {
				t.Fatal("expected HTTP/2")
			}
			body, err := io.ReadAll(response.Body)
			if err != nil || string(body) != payload {
				t.Fatalf("response unreadable: %q %v", body, err)
			}
		})
	}
}
