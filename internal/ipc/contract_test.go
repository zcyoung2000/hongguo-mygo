package ipc

import (
	"reflect"
	"testing"
)

func TestIPCMethodContractDoesNotExposeRuntimeWiring(t *testing.T) {
	disallowed := map[string]bool{
		"Close":                  true,
		"DeviceIdentity":         true,
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
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		if numOut := method.Type.NumOut(); numOut > 2 {
			t.Fatalf("IPC method %q has %d return values, want at most a value and an error", method.Name, numOut)
		}
	}
}

func TestIPCMethodSecondReturnValuesAreErrors(t *testing.T) {
	serviceType := reflect.TypeFor[*HongguoService]()
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		if method.Type.NumOut() == 2 && method.Type.Out(1) != reflect.TypeFor[error]() {
			t.Fatalf("IPC method %q returns (%s, %s), want (value, error)", method.Name, method.Type.Out(0), method.Type.Out(1))
		}
	}
}
