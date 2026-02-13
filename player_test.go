package mmlfm

import "testing"

func TestPlayerMasterVolumeRuntimeAPI(t *testing.T) {
	pl, err := NewPlayer(48000)
	if err != nil {
		t.Fatalf("new player: %v", err)
	}
	if got := pl.MasterVolume(); got != 1 {
		t.Fatalf("default master volume = %v, want 1", got)
	}
	pl.SetMasterVolume(0.35)
	if got := pl.MasterVolume(); got != 0.35 {
		t.Fatalf("master volume = %v, want 0.35", got)
	}
	pl.SetMasterVolume(-2)
	if got := pl.MasterVolume(); got != 0 {
		t.Fatalf("master volume should clamp to 0, got %v", got)
	}
}
