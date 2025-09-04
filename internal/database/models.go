package database

import (
	"time"
)

// Release represents a container image release in the database
type Release struct {
	ID            int       `json:"id" db:"id"`
	Namespace     string    `json:"namespace" db:"namespace"`
	WorkloadName  string    `json:"workload_name" db:"workload_name"`
	WorkloadType  string    `json:"workload_type" db:"workload_type"`
	ContainerName string    `json:"container_name" db:"container_name"`
	ImageRepo     string    `json:"image_repo" db:"image_repo"`
	ImageName     string    `json:"image_name" db:"image_name"`
	ImageTag      string    `json:"image_tag" db:"image_tag"`
	ImageSHA      string    `json:"image_sha" db:"image_sha"`
	ClientName    string    `json:"client_name" db:"client_name"`
	EnvName       string    `json:"env_name" db:"env_name"`
	FirstSeen     time.Time `json:"first_seen" db:"first_seen"`
	LastSeen      time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ImageFullPath returns the full image path constructed from repo, name, and tag
func (r *Release) ImageFullPath() string {
	if r.ImageRepo == "" {
		return r.ImageName + ":" + r.ImageTag
	}
	return r.ImageRepo + "/" + r.ImageName + ":" + r.ImageTag
}

// CurrentRelease represents the current state of deployed images
type CurrentRelease struct {
	Namespace     string    `json:"namespace"`
	WorkloadName  string    `json:"workload_name"`
	WorkloadType  string    `json:"workload_type"`
	ContainerName string    `json:"container_name"`
	ImageRepo     string    `json:"image_repo"`
	ImageName     string    `json:"image_name"`
	ImageTag      string    `json:"image_tag"`
	ImageSHA      string    `json:"image_sha"`
	ClientName    string    `json:"client_name"`
	EnvName       string    `json:"env_name"`
	LastSeen      time.Time `json:"last_seen"`
}

// ImageFullPath returns the full image path constructed from repo, name, and tag
func (r *CurrentRelease) ImageFullPath() string {
	if r.ImageRepo == "" {
		return r.ImageName + ":" + r.ImageTag
	}
	return r.ImageRepo + "/" + r.ImageName + ":" + r.ImageTag
}

// PendingRelease represents a release pending to be sent to master (used in slave mode)
type PendingRelease struct {
	ID            int       `json:"id" db:"id"`
	Namespace     string    `json:"namespace" db:"namespace"`
	WorkloadName  string    `json:"workload_name" db:"workload_name"`
	WorkloadType  string    `json:"workload_type" db:"workload_type"`
	ContainerName string    `json:"container_name" db:"container_name"`
	ImageRepo     string    `json:"image_repo" db:"image_repo"`
	ImageName     string    `json:"image_name" db:"image_name"`
	ImageTag      string    `json:"image_tag" db:"image_tag"`
	ImageSHA      string    `json:"image_sha" db:"image_sha"`
	ClientName    string    `json:"client_name" db:"client_name"`
	EnvName       string    `json:"env_name" db:"env_name"`
	FirstSeen     time.Time `json:"first_seen" db:"first_seen"`
	LastSeen      time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ImageFullPath returns the full image path constructed from repo, name, and tag
func (r *PendingRelease) ImageFullPath() string {
	if r.ImageRepo == "" {
		return r.ImageName + ":" + r.ImageTag
	}
	return r.ImageRepo + "/" + r.ImageName + ":" + r.ImageTag
}

// SlavePing represents a health ping from a slave instance
type SlavePing struct {
	ID           int       `json:"id" db:"id"`
	ClientName   string    `json:"client_name" db:"client_name"`
	EnvName      string    `json:"env_name" db:"env_name"`
	LastPingTime time.Time `json:"last_ping_time" db:"last_ping_time"`
	Status       string    `json:"status" db:"status"`
	SlaveVersion string    `json:"slave_version" db:"slave_version"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ReleaseHistory represents historical releases for a specific component
type ReleaseHistory struct {
	Releases []Release `json:"releases"`
	Total    int       `json:"total"`
}

// ComponentKey represents a unique component identifier
type ComponentKey struct {
	Namespace     string `json:"namespace"`
	WorkloadName  string `json:"workload_name"`
	ContainerName string `json:"container_name"`
}

// String returns a string representation of the component key
func (ck ComponentKey) String() string {
	return ck.Namespace + "/" + ck.WorkloadName + "/" + ck.ContainerName
}

// ParseImagePath parses a full image path into repository, name, and tag
func ParseImagePath(imagePath string) (repo, name, tag string) {
	// Default tag if not specified
	tag = "latest"

	// Split by tag separator
	parts := splitLast(imagePath, ":")
	if len(parts) == 2 {
		imagePath = parts[0]
		tag = parts[1]
	}

	// Split by repository separator
	repoParts := splitLast(imagePath, "/")
	if len(repoParts) == 2 {
		repo = repoParts[0]
		name = repoParts[1]
	} else {
		repo = ""
		name = imagePath
	}

	return repo, name, tag
}

// splitLast splits a string by the last occurrence of a separator
func splitLast(s, sep string) []string {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i:i+1] == sep {
			idx = i
			break
		}
	}

	if idx == -1 {
		return []string{s}
	}

	return []string{s[:idx], s[idx+1:]}
}
