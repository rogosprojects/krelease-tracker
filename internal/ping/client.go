package ping

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
)

// Client handles sending health pings to master
type Client struct {
	masterURL    string
	apiKey       string
	clientName   string
	envName      string
	slaveVersion string
	proxyURL     string
	tlsInsecure  bool
}

// New creates a new ping client
func New(masterURL, apiKey, clientName, envName, slaveVersion, proxyURL string, tlsInsecure bool) *Client {
	return &Client{
		masterURL:    masterURL,
		apiKey:       apiKey,
		clientName:   clientName,
		envName:      envName,
		slaveVersion: slaveVersion,
		proxyURL:     proxyURL,
		tlsInsecure:  tlsInsecure,
	}
}

// PingRequest represents the ping payload
type PingRequest struct {
	ClientName   string `json:"client_name"`
	EnvName      string `json:"env_name"`
	SlaveVersion string `json:"slave_version,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
}

// SendPing sends a health ping to the master
func (c *Client) SendPing(ctx context.Context) error {
	if c.masterURL == "" {
		return fmt.Errorf("master URL not configured")
	}

	pingData := PingRequest{
		ClientName:   c.clientName,
		EnvName:      c.envName,
		SlaveVersion: c.slaveVersion,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(pingData)
	if err != nil {
		return fmt.Errorf("failed to marshal ping data: %w", err)
	}

	requestURL := fmt.Sprintf("%s/api/ping", c.masterURL)
	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
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
		log.Println("Using proxy for ping")
	}

	// Configure TLS settings if insecure mode is enabled
	if c.tlsInsecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		log.Printf("TLS certificate verification disabled for ping (insecure mode)")
	}

	// Send request
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master returned status %d", resp.StatusCode)
	}

	return nil
}

// StartPingWorker starts a background worker that periodically sends pings
func (c *Client) StartPingWorker(ctx context.Context, interval time.Duration) {
	if c.masterURL == "" {
		log.Println("Ping worker disabled - MASTER_URL not configured")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting ping worker with interval %v to master %s", interval, c.masterURL)

	// Send initial ping
	if err := c.SendPing(ctx); err != nil {
		log.Printf("Initial ping failed: %v", err)
	} else {
		log.Printf("Initial ping sent successfully to master")
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Ping worker stopped")
			return
		case <-ticker.C:
			if err := c.SendPing(ctx); err != nil {
				log.Printf("Ping failed: %v", err)
			} else {
				log.Printf("Ping sent successfully to master (%s/%s)", c.clientName, c.envName)
			}
		}
	}
}

// SendPingWithRetry sends a ping with retry logic
func (c *Client) SendPingWithRetry(ctx context.Context, maxRetries int) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := c.SendPing(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		if i < maxRetries {
			// Wait before retry with exponential backoff
			waitTime := time.Duration(i+1) * 5 * time.Second
			log.Printf("Ping attempt %d failed, retrying in %v: %v", i+1, waitTime, err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}
	}

	return fmt.Errorf("ping failed after %d retries: %w", maxRetries, lastErr)
}
