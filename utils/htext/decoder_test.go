package htext

import (
	"testing"
)

func TestDecoder(t *testing.T) {
	src, err := Decode("hello")
	if err != nil || src != "hello" {
		t.Error("failed to decode 'hello'")
	}

	src, err = Decode("{{base64:aGVsbG8=}}")
	if err != nil || src != "hello" {
		t.Error("failed to decode 'hello'")
	}
}
