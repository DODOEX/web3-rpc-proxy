package helpers

import "runtime"

func Func(pc []uintptr) *runtime.Func {
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f
}
