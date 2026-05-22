package activator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// USBPresence activates each time a named USB volume appears on the
// system. The first time the volume is detected the activator returns;
// subsequent calls require the USB to be removed and reinserted before
// activating again — edge-triggered, not level-triggered. This lets a
// user without hand mobility "talk" by inserting their USB, and "stop"
// the next turn by removing it.
//
// On macOS, mounted volumes appear under /Volumes/<name>. On Linux the
// path is typically /media/<user>/<name> or /mnt/<name>; we accept any
// of those — caller can override via Paths.
type USBPresence struct {
	// VolumeName is the case-sensitive name of the USB volume that
	// signals activation. Default "AGENT".
	VolumeName string

	// Paths to scan. Default: ["/Volumes", "/media", "/mnt"].
	Paths []string

	// PollInterval between filesystem checks. Default 500ms.
	PollInterval time.Duration

	// wasPresent tracks the prior state so we only fire on rising edge.
	wasPresent bool
}

func NewUSBPresence(volumeName string) *USBPresence {
	return &USBPresence{
		VolumeName:   volumeName,
		Paths:        []string{"/Volumes", "/media", "/mnt"},
		PollInterval: 500 * time.Millisecond,
	}
}

func (USBPresence) Name() string { return "usb_presence" }

func (u *USBPresence) WaitForActivation(ctx context.Context) error {
	if u.VolumeName == "" {
		return fmt.Errorf("usb_presence: VolumeName is empty")
	}
	interval := u.PollInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Wait for the volume to be ABSENT before listening for the rising
	// edge — if the USB is already plugged in when WaitForActivation
	// is called for the second turn, we shouldn't double-trigger.
	if u.wasPresent {
		for {
			if !u.present() {
				u.wasPresent = false
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}

	// Now wait for the rising edge: not present -> present.
	for {
		if u.present() {
			u.wasPresent = true
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (u *USBPresence) present() bool {
	for _, base := range u.Paths {
		// On Linux /media/<user>/<NAME>; on Mac /Volumes/<NAME>. Match
		// both shapes by checking direct child AND one-level-deep.
		direct := filepath.Join(base, u.VolumeName)
		if _, err := os.Stat(direct); err == nil {
			return true
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(base, e.Name(), u.VolumeName)
			if _, err := os.Stat(candidate); err == nil {
				return true
			}
		}
	}
	return false
}

// SupportsBargeIn: removing+reinserting a USB mid-speech is awkward, so USB
// presence does not drive barge-in for now.
func (*USBPresence) SupportsBargeIn() bool { return false }
