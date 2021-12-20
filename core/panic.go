package core

import "fmt"

func Panic(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args))
}
