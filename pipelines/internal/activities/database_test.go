package activities

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestManagedDatabaseReachability(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	result, err := waitDatabaseEndpoint(ctx, listener.Addr().String(), func(int) {})
	if err != nil || result.Attempts != 1 || result.Address != listener.Addr().String() {
		t.Fatalf("reachability result=%+v err=%v", result, err)
	}
}

func TestManagedDatabaseReachabilityCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(t.Context())
	result, err := waitDatabaseEndpoint(ctx, address, func(int) { cancel() })
	if !errors.Is(err, context.Canceled) || result.Attempts != 1 {
		t.Fatalf("cancellation result=%+v err=%v", result, err)
	}
}
