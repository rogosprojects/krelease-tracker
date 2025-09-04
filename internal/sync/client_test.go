package sync

import (
	"testing"

	"release-tracker/internal/database"
)

func TestNew(t *testing.T) {
	// Create a mock database (nil is fine for this test)
	var db *database.DB

	// Test creating a new client with proxy and TLS settings
	client := New("https://master.example.com", "test-api-key", db, "http://proxy.example.com:8080", true)

	// Verify the client was created with the correct settings
	if client.masterURL != "https://master.example.com" {
		t.Errorf("Expected masterURL to be 'https://master.example.com', got '%s'", client.masterURL)
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("Expected apiKey to be 'test-api-key', got '%s'", client.apiKey)
	}

	if client.proxyURL != "http://proxy.example.com:8080" {
		t.Errorf("Expected proxyURL to be 'http://proxy.example.com:8080', got '%s'", client.proxyURL)
	}

	if !client.tlsInsecure {
		t.Errorf("Expected tlsInsecure to be true, got false")
	}
}

func TestNewWithoutProxyAndTLS(t *testing.T) {
	// Create a mock database (nil is fine for this test)
	var db *database.DB

	// Test creating a new client without proxy and TLS settings
	client := New("https://master.example.com", "test-api-key", db, "", false)

	// Verify the client was created with the correct settings
	if client.proxyURL != "" {
		t.Errorf("Expected proxyURL to be empty, got '%s'", client.proxyURL)
	}

	if client.tlsInsecure {
		t.Errorf("Expected tlsInsecure to be false, got true")
	}
}
