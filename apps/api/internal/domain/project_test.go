package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	now := time.Unix(0, 0)
	cases := []struct {
		name, pname, idea string
		orientation       Orientation
		wantErr           bool
		wantOrientation   Orientation
	}{
		{"defaults to portrait", "Runner", "one-button runner", "", false, Portrait},
		{"landscape ok", "Runner", "runner", Landscape, false, Landscape},
		{"empty name", "  ", "runner", "", true, ""},
		{"empty idea", "Runner", "", "", true, ""},
		{"bad orientation", "Runner", "runner", "diagonal", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewProject("p1", c.pname, c.idea, "", c.orientation, now)
			if c.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("want ErrInvalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Orientation != c.wantOrientation {
				t.Fatalf("orientation = %s, want %s", p.Orientation, c.wantOrientation)
			}
		})
	}
}
