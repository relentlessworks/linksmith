package models

import "time"

// Link represents a shortened URL.
type Link struct {
	Handle    string    `json:"handle"`
	URL       string    `json:"url"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	Clicks    int64     `json:"clicks"`
}

// Workspace represents a tenant in the system.
type Workspace struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Plan   string `json:"plan"`
}

// Token represents an auth token.
type Token struct {
	Value      string    `json:"value"`
	Workspace  string    `json:"workspace"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
