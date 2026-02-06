package repository

import (
	"time"
)

// timeLayout is the RFC3339 format for storing times in SQLite
const timeLayout = time.RFC3339

// parseTime parses a time string in RFC3339 format
func parseTime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

// formatTime returns the current time formatted as RFC3339
func formatTime() string {
	return time.Now().Format(timeLayout)
}
