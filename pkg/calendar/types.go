package calendar

// Calendar represents metadata about a calendar
type Calendar struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	TimeZone    string `json:"timezone,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
	AccessRole  string `json:"access_role,omitempty"`
}