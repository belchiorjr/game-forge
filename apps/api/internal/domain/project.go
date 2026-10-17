package domain

import (
	"fmt"
	"strings"
	"time"
)

type Orientation string

const (
	Portrait  Orientation = "portrait"
	Landscape Orientation = "landscape"
)

type Project struct {
	ID          string
	Name        string
	Idea        string
	Genre       string
	Orientation Orientation
	CreatedAt   time.Time
}

// NewProject validates the user input and applies defaults.
func NewProject(id, name, idea, genre string, orientation Orientation, now time.Time) (*Project, error) {
	name, idea = strings.TrimSpace(name), strings.TrimSpace(idea)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if idea == "" {
		return nil, fmt.Errorf("%w: idea is required", ErrInvalid)
	}
	if len(name) > 80 {
		return nil, fmt.Errorf("%w: name longer than 80 characters", ErrInvalid)
	}
	switch orientation {
	case "":
		orientation = Portrait
	case Portrait, Landscape:
	default:
		return nil, fmt.Errorf("%w: orientation must be portrait or landscape", ErrInvalid)
	}
	return &Project{ID: id, Name: name, Idea: idea, Genre: strings.TrimSpace(genre), Orientation: orientation, CreatedAt: now}, nil
}
