package assets

import (
	"os"
	"testing"
)

func TestManifestParses(t *testing.T) {
	data, err := os.ReadFile("../../assets/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("assets/manifest.json is invalid: %v", err)
	}
	for name, s := range m.Sprites {
		if s.File == "" || s.FrameW <= 0 || s.FrameH <= 0 {
			t.Errorf("sprite %q needs file, frame_w and frame_h", name)
		}
		if _, err := os.Stat("../../assets/" + s.File); err != nil {
			t.Errorf("sprite %q: %v", name, err)
		}
	}
}
