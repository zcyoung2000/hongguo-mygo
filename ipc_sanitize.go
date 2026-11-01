package main

import (
	"reflect"
)

// sanitizeIPC returns a JSON-safe copy for value-typed IPC results. Pointer
// targets are sanitized in place, so sanitize them before caching or sharing.
func sanitizeIPC[T any](value T) T {
	root := reflect.ValueOf(&value).Elem()
	sanitizeReflectValue(root)
	return value
}

// Upstream APIs are treated as untrusted text. MyGo rejects malformed UTF-8
// before it reaches the WebView, so remove invalid bytes at every IPC result
// boundary rather than making each DTO field responsible for its own cleanup.
func sanitizeReflectValue(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		if value.CanSet() {
			value.SetString(sanitizeUTF8(value.String()))
		}
	case reflect.Pointer:
		if !value.IsNil() {
			sanitizeReflectValue(value.Elem())
		}
	case reflect.Interface:
		if !value.IsNil() {
			sanitizeReflectValue(value.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			sanitizeReflectValue(value.Index(i))
		}
	case reflect.Map:
		if value.IsNil() {
			return
		}
		for _, key := range value.MapKeys() {
			oldValue := value.MapIndex(key)
			newValue := reflect.New(oldValue.Type()).Elem()
			newValue.Set(oldValue)
			sanitizeReflectValue(newValue)

			newKey := key
			if key.Kind() == reflect.String {
				newKey = reflect.New(key.Type()).Elem()
				newKey.SetString(sanitizeUTF8(key.String()))
			}
			if newKey.String() != key.String() {
				value.SetMapIndex(key, reflect.Value{})
			}
			value.SetMapIndex(newKey, newValue)
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			// Exported fields are the JSON contract; unexported fields cannot
			// be modified and do not participate in encoding.
			if value.Type().Field(i).IsExported() {
				sanitizeReflectValue(field)
			}
		}
	}
}
