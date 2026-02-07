package tui

import "fmt"

// formatHours formats hours as "Xh Ym"
func formatHours(hours float64) string {
	h := int(hours)
	m := int((hours - float64(h)) * 60)
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatMoney formats money as "$X,XXX.XX" with comma separators
func formatMoney(amount float64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	s := fmt.Sprintf("%.2f", amount)

	// Split at decimal point
	dotPos := len(s) - 3
	intPart := s[:dotPos]
	decPart := s[dotPos:]

	// Add commas to integer part
	result := make([]byte, 0, len(intPart)+len(intPart)/3)
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	prefix := "$"
	if negative {
		prefix = "-$"
	}
	return prefix + string(result) + decPart
}

// truncateStr truncates a string to the specified length with ellipsis
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
