package tui

// SwitchScreenMsg requests a screen change
type SwitchScreenMsg struct {
	Screen Screen
}

// RefreshDataMsg requests data refresh
type RefreshDataMsg struct{}

// ErrorMsg carries error information
type ErrorMsg struct {
	Err error
}

// OpenNewClientFormMsg tells the clients screen to open the new client form
type OpenNewClientFormMsg struct{}

// firstRunCheckMsg reports whether the database has any clients
type firstRunCheckMsg struct {
	hasClients bool
}

// (Timer ticks are managed by individual screen implementations as needed)
