package config

import (
	"log"
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	Port               string
	DatabasePath       string
	Namespaces         []string
	InCluster          bool
	KubeconfigPath     string
	CollectionInterval int      // in minutes
	APIKeys            []string // API keys for authentication
	EnvName            string   // Environment name for badges
	ClientName         string   // Client name for releases
	BasePath           string   // Base path for serving (e.g., "/tracker")
	Mode               string   // Application mode: "master" or "slave"
	MasterURL          string   // Master URL for sync (slave mode only)
	MasterAPIKey       string   // Master API key for sync (slave mode only)
	SyncInterval       int      // Sync interval in minutes (slave mode only)
	ProxyURL           string   // HTTP/HTTPS proxy URL for sync requests (slave mode only)
	TLSInsecure        bool     // Skip TLS certificate verification for sync requests (slave mode only)
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabasePath:       getEnv("DATABASE_PATH", "/data/releases.db"),
		InCluster:          getEnv("IN_CLUSTER", "true") == "true",
		KubeconfigPath:     getEnv("KUBECONFIG", ""),
		CollectionInterval: getEnvInt("COLLECTION_INTERVAL", 60), // 1 hour default
		EnvName:            getEnv("ENV_NAME", "master"),
		ClientName:         getEnv("CLIENT_NAME", "master"),
		BasePath:           normalizeBasePath(getEnv("BASE_PATH", "")),
		Mode:               getEnv("MODE", "slave"), // Default to slave mode
		MasterURL:          getEnv("MASTER_URL", ""),
		MasterAPIKey:       getEnv("MASTER_API_KEY", ""),
		SyncInterval:       getEnvInt("SYNC_INTERVAL", 5), // 5 minutes default
		ProxyURL:           getEnv("PROXY_URL", ""),
		TLSInsecure:        getEnv("TLS_INSECURE", "false") == "true",
	}

	// Parse namespaces from environment variable or use default
	namespacesStr := getEnv("NAMESPACES", "default")
	config.Namespaces = strings.Split(namespacesStr, ",")
	for i := range config.Namespaces {
		config.Namespaces[i] = strings.TrimSpace(config.Namespaces[i])
	}

	// Parse API keys from environment variable
	apiKeysStr := getEnv("API_KEYS", "")
	if apiKeysStr != "" {
		keys := strings.Split(apiKeysStr, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if isValidAPIKey(key) {
				config.APIKeys = append(config.APIKeys, key)
			} else if key != "" {
				log.Printf("Warning: Invalid API key format (key must be at least 32 characters and contain only alphanumeric, hyphens, and underscores): %s...", key[:min(8, len(key))])
			}
		}
		if len(config.APIKeys) == 0 {
			log.Println("Warning: No valid API keys found, authentication will be disabled")
		} else {
			log.Printf("Loaded %d valid API key(s)", len(config.APIKeys))
		}
	} else {
		log.Println("No API keys configured, authentication disabled")
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue := parseInt(value); intValue > 0 {
			return intValue
		}
	}
	return defaultValue
}

func parseInt(s string) int {
	var result int
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		} else {
			return 0
		}
	}
	return result
}

// isValidAPIKey validates API key format
func isValidAPIKey(key string) bool {
	// API key must be at least 32 characters
	if len(key) < 32 {
		return false
	}

	// API key should contain only alphanumeric characters, hyphens, and underscores
	for _, char := range key {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizeBasePath ensures the base path starts with / and doesn't end with /
func normalizeBasePath(path string) string {
	if path == "" {
		return ""
	}

	// Ensure it starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove trailing / unless it's just "/"
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	return path
}
