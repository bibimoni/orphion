package torrent

import "testing"

func TestNewAnacrolixConfigDisablesDefaultPortForwarding(t *testing.T) {
	cfg := newAnacrolixConfig("/tmp/orphion-test")
	if !cfg.NoDefaultPortForwarding {
		t.Fatal("NoDefaultPortForwarding = false, want true")
	}
}
