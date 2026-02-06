package domain

import (
	"errors"
	"strings"
	"time"
)

type Client struct {
	ID         int64
	Name       string
	Email      string
	HourlyRate float64
	Notes      string
	IsArchived bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewClient creates a new client with required fields
func NewClient(name string, hourlyRate float64) *Client {
	now := time.Now()
	return &Client{
		Name:       strings.TrimSpace(name),
		HourlyRate: hourlyRate,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Validate returns an error if the client is invalid
func (c *Client) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("client name is required")
	}
	if c.HourlyRate < 0 {
		return errors.New("hourly rate cannot be negative")
	}
	return nil
}
