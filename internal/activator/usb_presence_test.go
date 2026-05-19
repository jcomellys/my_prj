package activator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUSBPresence_RisingEdge(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "AGENT")

	u := &USBPresence{
		VolumeName:   "AGENT",
		Paths:        []string{tmp},
		PollInterval: 20 * time.Millisecond,
	}

	// Spawn a goroutine that creates the volume after ~50ms.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.MkdirAll(target, 0o755)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := u.WaitForActivation(ctx); err != nil {
		t.Fatalf("WaitForActivation: %v", err)
	}

	// Second call must NOT fire while volume is still present.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	err := u.WaitForActivation(ctx2)
	if err == nil {
		t.Fatal("expected timeout: USB still inserted, no edge")
	}

	// Remove the volume, then re-create it. The second activation should fire.
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("remove: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.MkdirAll(target, 0o755)
	}()
	ctx3, cancel3 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel3()
	if err := u.WaitForActivation(ctx3); err != nil {
		t.Fatalf("second activation failed: %v", err)
	}
}

func TestUSBPresence_RequiresVolumeName(t *testing.T) {
	u := &USBPresence{}
	err := u.WaitForActivation(context.Background())
	if err == nil {
		t.Fatal("expected error when VolumeName empty")
	}
}
