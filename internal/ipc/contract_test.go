package ipc

import (
	"reflect"
	"testing"
)

func TestIPCMethodContractDoesNotExposeRuntimeWiring(t *testing.T) {
	disallowed := map[string]bool{
		"ResetDeviceWithContext": true,
		"SetAuthTransport":       true,
		"SetDeviceProfileStore":  true,
	}
	serviceType := reflect.TypeFor[*HongguoService]()
	if serviceType.NumMethod() < 50 {
		t.Fatalf("IPC method count = %d, want the established contract", serviceType.NumMethod())
	}
	for i := 0; i < serviceType.NumMethod(); i++ {
		name := serviceType.Method(i).Name
		if disallowed[name] {
			t.Fatalf("runtime wiring method %q is exposed to IPC", name)
		}
	}
}
