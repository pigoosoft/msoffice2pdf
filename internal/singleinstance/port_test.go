package singleinstance

import (
	"net"
	"testing"
)

func TestCheckPortFree_OK(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	if err := CheckPortFree(port); err != nil {
		t.Fatalf("expected free port %d: %v", port, err)
	}
}

func TestCheckPortFree_InUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if err := CheckPortFree(port); err == nil {
		t.Fatalf("expected port %d in use", port)
	}
}

func TestCheckPortFree_Invalid(t *testing.T) {
	if err := CheckPortFree(0); err == nil {
		t.Fatal("expected error for port 0")
	}
}
