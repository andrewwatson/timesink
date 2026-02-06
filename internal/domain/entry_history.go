package domain

import "time"

type EntryHistory struct {
	ID           int64
	EntryID      int64
	FieldName    string
	OldValue     string
	NewValue     string
	ChangeReason string
	ChangedAt    time.Time
}

// NewEntryHistory creates a history record for a field change
func NewEntryHistory(entryID int64, fieldName, oldValue, newValue, reason string) *EntryHistory {
	return &EntryHistory{
		EntryID:      entryID,
		FieldName:    fieldName,
		OldValue:     oldValue,
		NewValue:     newValue,
		ChangeReason: reason,
		ChangedAt:    time.Now(),
	}
}
