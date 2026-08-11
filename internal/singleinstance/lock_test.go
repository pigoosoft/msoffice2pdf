package singleinstance

import (
	"errors"
	"testing"
)

func TestAcquire_SecondFails(t *testing.T) {
	first, err := Acquire()
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer func() { _ = first.Release() }()

	second, err := Acquire()
	if !errors.Is(err, ErrAlreadyRunning) {
		if second != nil {
			_ = second.Release()
		}
		t.Fatalf("second Acquire: got %v, want ErrAlreadyRunning", err)
	}
}
