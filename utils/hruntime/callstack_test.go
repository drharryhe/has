package hruntime

import (
	"fmt"
	"testing"
)

func TestCallStack(t *testing.T) {
	foo()
}

func foo() {
	subFool()
}

func subFool() {
	PrintCallStack(32, 3)
	fmt.Println("---------")
	ss := SprintfCallers("%+v", 32, 3)
	for _, s := range ss {
		fmt.Println(s)
	}
}
