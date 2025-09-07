package sync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"krelease-tracker/internal/database"
)

// Client handles syncing pending releases to master
type Client struct {
	masterURL   string
	apiKey      string
	db          *database.DB
	proxyURL    string
	tlsInsecure bool
}

// New creates a new sync client
func New(masterURL, apiKey string, db *database.DB, proxyURL string, tlsInsecure bool) *Client {
	return &Client{
		masterURL:   masterURL,
		apiKey:      apiKey,
		db:          db,
		proxyURL:    proxyURL,
		tlsInsecure: tlsInsecure,
	}
}

// SyncPendingReleases sends all pending releases to master and removes them on success
func (c *Client) SyncPendingReleases(ctx context.Context) error {
	pendingReleases, err := c.db.GetPendingReleases()
	if err != nil {
		return fmt.Errorf("failed to get pending releases: %w", err)
	}

	if len(pendingReleases) == 0 {
		log.Println("No pending releases to sync")
		return nil
	}

	log.Printf("Syncing %d pending releases to master", len(pendingReleases))

	for _, release := range pendingReleases {
		if err := c.syncSingleRelease(ctx, &release); err != nil {
			log.Printf("Failed to sync release %d: %v", release.ID, err)
			continue
		}

		// Remove from pending releases on successful sync
		if err := c.db.DeletePendingRelease(release.ID); err != nil {
			log.Printf("Failed to delete pending release %d: %v", release.ID, err)
		} else {
			log.Printf("Successfully synced and removed pending release %d", release.ID)
		}
	}

	return nil
}

// syncSingleRelease sends a single release to the master
func (c *Client) syncSingleRelease(ctx context.Context, release *database.PendingRelease) error {
	// Convert PendingRelease to the format expected by the manual collect API
	requestBody := map[string]interface{}{
		"image_tag":   release.ImageTag,
		"image_sha":   release.ImageSHA,
		"image_repo":  release.ImageRepo,
		"image_name":  release.ImageName,
		"client_name": release.ClientName,
		"env_name":    release.EnvName,
		"released_at": release.LastSeen.UTC(),
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the URL for the manual collect endpoint
	requestURL := fmt.Sprintf("%s/api/collect/%s/%s/%s/%s",
		c.masterURL,
		release.Namespace,
		release.WorkloadType,
		release.WorkloadName,
		release.ContainerName,
	)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "PUT", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	// Create HTTP client with custom transport for proxy and TLS settings
	transport := &http.Transport{}

	// Configure proxy if provided
	if c.proxyURL != "" {
		proxyURL, err := url.Parse(c.proxyURL)
		if err != nil {
			return fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		log.Println("Using proxy for sync")
	}

	// Configure TLS settings if insecure mode is enabled
	if c.tlsInsecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		log.Println("TLS certificate verification disabled (insecure mode)")
	}

	// Send request
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master returned status %d", resp.StatusCode)
	}

	return nil
}

// StartSyncWorker starts a background worker that periodically syncs pending releases
func (c *Client) StartSyncWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting sync worker with interval %v", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Sync worker stopped")
			return
		case <-ticker.C:
			if err := c.SyncPendingReleases(ctx); err != nil {
				log.Printf("Sync failed: %v", err)
			}
		}
	}
}
