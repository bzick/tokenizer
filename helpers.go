package tokenizer

import (
	"reflect"
	"runtime"
	"unsafe"
)

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// s2b converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func s2b(s string) (b []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len

	runtime.KeepAlive(&s)

	return b
}

func isNumberByte(b byte) bool {
	return '0' <= b && b <= '9'
}

func bytesStarts(prefix []byte, b []byte) bool {
	if len(prefix) > len(b) {
		return false
	}
	return b2s(prefix) == b2s(b[0:len(prefix)])
}

func bytesEnds(suffix []byte, b []byte) bool {
	if len(suffix) > len(b) {
		return false
	}
	return b2s(suffix) == b2s(b[len(b)-len(suffix):])
}
