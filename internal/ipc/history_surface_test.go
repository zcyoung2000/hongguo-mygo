package ipc

import (
	"reflect"
	"testing"
)

func TestHistoryDeletionIsNotExposedThroughIPC(t *testing.T) {
	service := reflect.TypeOf((*HongguoService)(nil))
	for _, name := range []string{"DeleteHistory", "ClearHistory"} {
		if _, ok := service.MethodByName(name); ok {
			t.Fatalf("destructive history method %s remains exposed", name)
		}
	}
	for _, name := range []string{"SaveProgress", "GetHistory", "RefreshHistory"} {
		if _, ok := service.MethodByName(name); !ok {
			t.Fatalf("history sync method %s missing", name)
		}
	}
}
