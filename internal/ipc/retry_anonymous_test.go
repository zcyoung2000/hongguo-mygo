package ipc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestRetryAnonymousSkipsWhenParentBudgetIsExhausted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	cancel()
	calls := 0
	op := func(context.Context) (string, error) {
		calls++
		return "", sdk.ErrAnonymousLimited
	}

	_, err := retryAnonymous(&HongguoService{}, ctx, op)
	if !errors.Is(err, sdk.ErrAnonymousLimited) {
		t.Fatalf("retryAnonymous err = %v, want anonymous limit", err)
	}
	if calls != 1 {
		t.Fatalf("anonymous retry calls = %d, want 1", calls)
	}
}

func TestRetryAnonymousUsesIndependentBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	calls := 0
	op := func(retryCtx context.Context) (string, error) {
		calls++
		if calls == 1 {
			return "", sdk.ErrAnonymousLimited
		}
		if _, ok := retryCtx.Deadline(); !ok {
			t.Error("anonymous retry has no independent deadline")
		}
		return "fresh-device", nil
	}
	resets := 0
	service := &HongguoService{resetDevice: func(context.Context) error {
		resets++
		return nil
	}}

	value, err := retryAnonymous(service, ctx, op)
	if err != nil || value != "fresh-device" {
		t.Fatalf("retryAnonymous = %q, %v; want fresh-device, nil", value, err)
	}
	if calls != 2 || resets != 1 {
		t.Fatalf("calls/resets = %d/%d, want 2/1", calls, resets)
	}
}
