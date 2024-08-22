package types

import "reflect"

func Uint16(i int16) uint16 {
	return uint16(i)
}

func Uint32(i int32) uint32 {
	return uint32(i)
}

func Uint64(i int64) uint64 {
	return uint64(i)
}

func Uint(i int) uint {
	return uint(i)
}

func PtrUint16(i int16) *uint16 {
	var u = Uint16(i)
	return &u
}

func PtrUint32(i int32) *uint32 {
	var u = Uint32(i)
	return &u
}

func PtrUint64(i int64) *uint64 {
	var u = Uint64(i)
	return &u
}

func PtrUint(i int) *uint {
	var u = Uint(i)
	return &u
}
func PtrBool(b bool) *bool {
	return &b
}

func PtrString(s string) *string {
	return &s
}

func IsArray(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Array
}

func IsSlice(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Slice
}
