package calendar

import (
	"encoding/json"
	"testing"
)

func TestCalendar_JSONSerialization(t *testing.T) {
	calendar := &Calendar{
		ID:          "test-calendar-id",
		Name:        "Test Calendar",
		Description: "A test calendar",
		TimeZone:    "America/New_York",
		Primary:     true,
		AccessRole:  "owner",
	}

	// Test marshaling
	data, err := json.Marshal(calendar)
	if err != nil {
		t.Fatalf("Failed to marshal calendar: %v", err)
	}

	// Test unmarshaling
	var unmarshaledCalendar Calendar
	err = json.Unmarshal(data, &unmarshaledCalendar)
	if err != nil {
		t.Fatalf("Failed to unmarshal calendar: %v", err)
	}

	// Compare fields
	if unmarshaledCalendar.ID != calendar.ID {
		t.Errorf("Expected ID %s, got %s", calendar.ID, unmarshaledCalendar.ID)
	}
	if unmarshaledCalendar.Name != calendar.Name {
		t.Errorf("Expected Name %s, got %s", calendar.Name, unmarshaledCalendar.Name)
	}
	if unmarshaledCalendar.Description != calendar.Description {
		t.Errorf("Expected Description %s, got %s", calendar.Description, unmarshaledCalendar.Description)
	}
	if unmarshaledCalendar.TimeZone != calendar.TimeZone {
		t.Errorf("Expected TimeZone %s, got %s", calendar.TimeZone, unmarshaledCalendar.TimeZone)
	}
	if unmarshaledCalendar.Primary != calendar.Primary {
		t.Errorf("Expected Primary %t, got %t", calendar.Primary, unmarshaledCalendar.Primary)
	}
	if unmarshaledCalendar.AccessRole != calendar.AccessRole {
		t.Errorf("Expected AccessRole %s, got %s", calendar.AccessRole, unmarshaledCalendar.AccessRole)
	}
}

func TestCalendar_JSONSerialization_OmitEmpty(t *testing.T) {
	// Test with minimal fields (omitempty should exclude empty optional fields)
	calendar := &Calendar{
		ID:   "minimal-calendar",
		Name: "Minimal Calendar",
	}

	data, err := json.Marshal(calendar)
	if err != nil {
		t.Fatalf("Failed to marshal calendar: %v", err)
	}

	// Parse as generic map to check which fields are present
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	if err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	// Required fields should be present
	if _, exists := jsonMap["id"]; !exists {
		t.Error("Expected 'id' field to be present")
	}
	if _, exists := jsonMap["name"]; !exists {
		t.Error("Expected 'name' field to be present")
	}

	// Optional fields with omitempty should not be present when empty
	if _, exists := jsonMap["description"]; exists {
		t.Error("Expected 'description' field to be omitted when empty")
	}
	if _, exists := jsonMap["timezone"]; exists {
		t.Error("Expected 'timezone' field to be omitted when empty")
	}
	if _, exists := jsonMap["access_role"]; exists {
		t.Error("Expected 'access_role' field to be omitted when empty")
	}

	// Primary field with omitempty should not be present when false (zero value)
	if _, exists := jsonMap["primary"]; exists {
		t.Error("Expected 'primary' field to be omitted when false")
	}
}

func TestCalendar_JSONSerialization_FullRoundTrip(t *testing.T) {
	originalJSON := `{
		"id": "test-id-123",
		"name": "My Test Calendar",
		"description": "This is a test calendar description",
		"timezone": "Europe/London",
		"primary": true,
		"access_role": "reader"
	}`

	var calendar Calendar
	err := json.Unmarshal([]byte(originalJSON), &calendar)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify all fields were parsed correctly
	if calendar.ID != "test-id-123" {
		t.Errorf("Expected ID 'test-id-123', got %s", calendar.ID)
	}
	if calendar.Name != "My Test Calendar" {
		t.Errorf("Expected Name 'My Test Calendar', got %s", calendar.Name)
	}
	if calendar.Description != "This is a test calendar description" {
		t.Errorf("Expected Description 'This is a test calendar description', got %s", calendar.Description)
	}
	if calendar.TimeZone != "Europe/London" {
		t.Errorf("Expected TimeZone 'Europe/London', got %s", calendar.TimeZone)
	}
	if !calendar.Primary {
		t.Error("Expected Primary to be true")
	}
	if calendar.AccessRole != "reader" {
		t.Errorf("Expected AccessRole 'reader', got %s", calendar.AccessRole)
	}

	// Marshal back to JSON and verify it's valid
	data, err := json.Marshal(calendar)
	if err != nil {
		t.Fatalf("Failed to marshal calendar back to JSON: %v", err)
	}

	// Parse the marshaled JSON to verify it's valid
	var verifyCalendar Calendar
	err = json.Unmarshal(data, &verifyCalendar)
	if err != nil {
		t.Fatalf("Failed to unmarshal marshaled JSON: %v", err)
	}

	// Should be identical to original
	if verifyCalendar != calendar {
		t.Error("Round-trip marshaling/unmarshaling should preserve all values")
	}
}

func TestCalendar_EmptyStruct(t *testing.T) {
	var calendar Calendar

	// Test that zero values work correctly
	if calendar.ID != "" {
		t.Errorf("Expected empty ID, got %s", calendar.ID)
	}
	if calendar.Name != "" {
		t.Errorf("Expected empty Name, got %s", calendar.Name)
	}
	if calendar.Description != "" {
		t.Errorf("Expected empty Description, got %s", calendar.Description)
	}
	if calendar.TimeZone != "" {
		t.Errorf("Expected empty TimeZone, got %s", calendar.TimeZone)
	}
	if calendar.Primary {
		t.Error("Expected Primary to be false")
	}
	if calendar.AccessRole != "" {
		t.Errorf("Expected empty AccessRole, got %s", calendar.AccessRole)
	}

	// Test JSON marshaling of empty struct
	data, err := json.Marshal(calendar)
	if err != nil {
		t.Fatalf("Failed to marshal empty calendar: %v", err)
	}

	// Should only contain required fields (without omitempty)
	expectedJSON := `{"id":"","name":""}`
	var expected, actual map[string]interface{}

	json.Unmarshal([]byte(expectedJSON), &expected)
	json.Unmarshal(data, &actual)

	if len(actual) != len(expected) {
		t.Errorf("Expected JSON with %d fields, got %d fields", len(expected), len(actual))
	}
}