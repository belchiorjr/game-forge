// Package assets loads assets/manifest.json. Code reads sprite sizes from here, never hardcodes them.
package assets

import "encoding/json"

type Animation struct {
	Frames []int `json:"frames"`
	FPS    int   `json:"fps"`
	Loop   bool  `json:"loop"`
}

type Source struct {
	Generator string `json:"generator"`
	Model     string `json:"model"`
	License   string `json:"license"`
	Prompt    string `json:"prompt"`
}

type Sprite struct {
	File       string               `json:"file"`
	FrameW     int                  `json:"frame_w"`
	FrameH     int                  `json:"frame_h"`
	Animations map[string]Animation `json:"animations"`
	Source     Source               `json:"source"`
}

type Manifest struct {
	Sprites map[string]Sprite `json:"sprites"`
}

func Parse(data []byte) (Manifest, error) {
	var m Manifest
	err := json.Unmarshal(data, &m)
	return m, err
}
