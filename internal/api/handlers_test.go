package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"krelease-tracker/internal/config"
	"krelease-tracker/internal/database"
)

// DatabaseInterface defines the interface for database operations
type DatabaseInterface interface {
	UpsertRelease(release *database.Release) error
	GetCurrentReleases() ([]database.CurrentRelease, error)
	GetReleaseHistory(namespace, workloadName, containerName string) (*database.ReleaseHistory, error)
	GetCurrentReleaseByWorkload(workloadType, workloadName, containerName string) (*database.CurrentRelease, error)
	CleanupOldReleases() error
}

// MockDB implements a simple in-memory database for testing
type MockDB struct {
	releases []database.Release
}

func (m *MockDB) UpsertRelease(release *database.Release) error {
	// Simple implementation - just append for testing
	m.releases = append(m.releases, *release)
	return nil
}

func (m *MockDB) GetCurrentReleases() ([]database.CurrentRelease, error) {
	var current []database.CurrentRelease
	for _, r := range m.releases {
		current = append(current, database.CurrentRelease{
			Namespace:     r.Namespace,
			WorkloadName:  r.WorkloadName,
			WorkloadType:  r.WorkloadType,
			ContainerName: r.ContainerName,
			ImageRepo:     r.ImageRepo,
			ImageName:     r.ImageName,
			ImageTag:      r.ImageTag,
			ImageSHA:      r.ImageSHA,
			ClientName:    r.ClientName,
			EnvName:       r.EnvName,
			LastSeen:      r.LastSeen,
		})
	}
	return current, nil
}

func (m *MockDB) GetReleaseHistory(namespace, workloadName, containerName string) (*database.ReleaseHistory, error) {
	return &database.ReleaseHistory{}, nil
}

func (m *MockDB) GetCurrentReleaseByWorkload(workloadType, workloadName, containerName string) (*database.CurrentRelease, error) {
	return nil, nil
}

func (m *MockDB) CleanupOldReleases() error {
	return nil
}

func TestManualCollectRequestStructure(t *testing.T) {
	// Test the ManualCollectRequest struct and JSON parsing
	tests := []struct {
		name        string
		jsonInput   string
		expectError bool
		expected    ManualCollectRequest
	}{
		{
			name:        "Valid request with release_version only",
			jsonInput:   `{"image_tag": "1.21.0"}`,
			expectError: false,
			expected: ManualCollectRequest{
				ImageTag:   "1.21.0",
				ImageSHA:   "",
				ReleasedAt: nil,
			},
		},
		{
			name:        "Valid request with both fields",
			jsonInput:   `{"image_tag": "13.4", "released_at": "2023-12-01T10:30:00Z"}`,
			expectError: false,
			expected: ManualCollectRequest{
				ImageTag:   "13.4",
				ImageSHA:   "",
				ReleasedAt: func() *time.Time { t, _ := time.Parse(time.RFC3339, "2023-12-01T10:30:00Z"); return &t }(),
			},
		},
		{
			name:        "Valid request with image_sha",
			jsonInput:   `{"image_tag": "v1.2.3", "image_sha": "abc123def456789012345678901234567890123456789012345678901234567890"}`,
			expectError: false,
			expected: ManualCollectRequest{
				ImageTag:   "v1.2.3",
				ImageSHA:   "abc123def456789012345678901234567890123456789012345678901234567890",
				ReleasedAt: nil,
			},
		},
		{
			name:        "Invalid JSON",
			jsonInput:   `{"image_tag": "1.21.0"`,
			expectError: true,
		},
		{
			name:        "Empty release_version",
			jsonInput:   `{"image_tag": ""}`,
			expectError: false,
			expected: ManualCollectRequest{
				ImageTag:   "",
				ReleasedAt: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ManualCollectRequest
			err := json.Unmarshal([]byte(tt.jsonInput), &req)

			if tt.expectError && err == nil {
				t.Error("Expected JSON parsing error, but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected JSON parsing error: %v", err)
			}

			if !tt.expectError && err == nil {
				if req.ImageTag != tt.expected.ImageTag {
					t.Errorf("Expected release_version %s, got %s", tt.expected.ImageTag, req.ImageTag)
				}

				if tt.expected.ReleasedAt != nil && req.ReleasedAt != nil {
					if !req.ReleasedAt.Equal(*tt.expected.ReleasedAt) {
						t.Errorf("Expected released_at %v, got %v", *tt.expected.ReleasedAt, *req.ReleasedAt)
					}
				} else if tt.expected.ReleasedAt != req.ReleasedAt {
					t.Errorf("Expected released_at %v, got %v", tt.expected.ReleasedAt, req.ReleasedAt)
				}
			}
		})
	}
}

func TestManualCollectRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		requestBody string
		expectError bool
	}{
		{
			name:        "Valid JSON with release_version",
			requestBody: `{"image_tag": "1.21.0"}`,
			expectError: false,
		},
		{
			name:        "Valid JSON with release_version and released_at",
			requestBody: `{"image_tag": "1.21.0", "released_at": "2023-12-01T10:30:00Z"}`,
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			requestBody: `{"image_tag": "1.21.0"`,
			expectError: true,
		},
		{
			name:        "Empty JSON",
			requestBody: `{}`,
			expectError: false, // JSON parsing succeeds, but validation should fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ManualCollectRequest
			err := json.Unmarshal([]byte(tt.requestBody), &req)

			if tt.expectError && err == nil {
				t.Error("Expected JSON parsing error, but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected JSON parsing error: %v", err)
			}
		})
	}
}

func TestImagePathParsing(t *testing.T) {
	// Test the database.ParseImagePath function used in the manual collect handler
	tests := []struct {
		name         string
		imagePath    string
		expectedRepo string
		expectedName string
		expectedTag  string
	}{
		{
			name:         "Simple image with tag",
			imagePath:    "nginx:1.21.0",
			expectedRepo: "",
			expectedName: "nginx",
			expectedTag:  "1.21.0",
		},
		{
			name:         "Image with repository and tag",
			imagePath:    "docker.io/library/nginx:1.21.0",
			expectedRepo: "docker.io/library",
			expectedName: "nginx",
			expectedTag:  "1.21.0",
		},
		{
			name:         "Image without tag (defaults to latest)",
			imagePath:    "nginx",
			expectedRepo: "",
			expectedName: "nginx",
			expectedTag:  "latest",
		},
		{
			name:         "Private registry with tag",
			imagePath:    "registry.company.com/myapp:v1.2.3",
			expectedRepo: "registry.company.com",
			expectedName: "myapp",
			expectedTag:  "v1.2.3",
		},
		{
			name:         "Complex registry path",
			imagePath:    "quay.io/organization/project:sha-abc123",
			expectedRepo: "quay.io/organization",
			expectedName: "project",
			expectedTag:  "sha-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, name, tag := database.ParseImagePath(tt.imagePath)

			if repo != tt.expectedRepo {
				t.Errorf("Expected repo %s, got %s", tt.expectedRepo, repo)
			}

			if name != tt.expectedName {
				t.Errorf("Expected name %s, got %s", tt.expectedName, name)
			}

			if tag != tt.expectedTag {
				t.Errorf("Expected tag %s, got %s", tt.expectedTag, tag)
			}
		})
	}
}

func TestHandleCollectResponseFormat(t *testing.T) {
	// Test that the collect endpoint returns the expected response format
	// without actually running the collection process

	// Create a minimal server for testing response format only
	server := &Server{
		config: &config.Config{
			EnvName:    "test",
			ClientName: "test-client",
		},
	}

	// Create request
	req, err := http.NewRequest("POST", "/api/collect", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create response recorder
	rr := httptest.NewRecorder()

	// Record start time
	startTime := time.Now()

	// Call handler
	server.handleCollect(rr, req)

	// Check that response was immediate (should be very fast since it just returns)
	elapsed := time.Since(startTime)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Response took too long: %v (expected immediate response)", elapsed)
	}

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Could not parse response JSON: %v", err)
	}

	// Verify response structure
	if response["status"] != "accepted" {
		t.Errorf("Expected status 'accepted', got %v", response["status"])
	}

	if response["message"] != "Collection process started successfully" {
		t.Errorf("Expected message 'Collection process started successfully', got %v", response["message"])
	}

	if _, exists := response["timestamp"]; !exists {
		t.Error("Expected timestamp in response")
	}

	// Give any background goroutine a moment to start (though it will fail due to nil k8s client)
	time.Sleep(10 * time.Millisecond)
}
