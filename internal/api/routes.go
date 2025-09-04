package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// Create base router (with or without base path)
	var baseRouter *mux.Router
	if s.config.BasePath != "" {
		baseRouter = s.router.PathPrefix(s.config.BasePath).Subrouter()
	} else {
		baseRouter = s.router
	}

	// API routes with authentication middleware
	api := baseRouter.PathPrefix("/api").Subrouter()

	// Apply authentication middleware to API routes if API keys are configured
	if len(s.apiKeys) > 0 {
		api.Use(s.authMiddleware)
	}

	api.HandleFunc("/collect", s.handleCollect).Methods("POST")
	api.HandleFunc("/collect/{namespace}/{workload-kind}/{workload-name}/{container}", s.handleManualCollect).Methods("PUT")

	api.HandleFunc("/releases/current", s.handleCurrentReleases).Methods("GET")
	api.HandleFunc("/releases/history/{client}/{env}/{namespace}/{workload}/{container}", s.handleReleaseHistory).Methods("GET")
	api.HandleFunc("/clients-environments", s.handleClientsEnvironments).Methods("GET")
	api.HandleFunc("/ping", s.handlePing).Methods("POST")
	api.HandleFunc("/config", s.handleConfig).Methods("GET")

	// Health check (no authentication required)
	baseRouter.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Badge endpoint with URL-based API key authentication
	baseRouter.HandleFunc("/badges/{api-key}/{client}/{env}/{workload-kind}/{workload-name}/{container}", s.handleBadgeWithAuth).Methods("GET")

	// Static files (no authentication required)
	if s.config.BasePath != "" {
		// When using base path, we need to strip the base path from static file requests
		staticHandler := http.StripPrefix(s.config.BasePath, http.FileServer(http.Dir("./web/static/")))
		baseRouter.PathPrefix("/").Handler(staticHandler)
	} else {
		// No base path, serve files directly
		baseRouter.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/static/")))
	}
}

// CORS middleware for development
func (s *Server) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// authMiddleware validates API keys for protected routes and sets client context
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := s.extractAPIKey(r)

		if apiKey == "" {
			s.sendUnauthorizedResponse(w, "Missing API key")
			return
		}

		// Parse API key to determine type and extract components
		clientName, clientAuth, isAdmin := parseAPIKey(apiKey)

		// Validate API key access
		if !s.validateAPIKeyAccess(clientName, clientAuth, isAdmin) {
			// Log failed authentication attempt with sanitized key
			keyPreview := apiKey[:min(8, len(apiKey))] + "..."
			log.Printf("Authentication failed for %s %s (key: %s)", r.Method, r.URL.Path, keyPreview)
			s.sendUnauthorizedResponse(w, "Invalid API key")
			return
		}

		// Set client context in request headers for downstream handlers
		if !isAdmin && clientName != "" {
			r.Header.Set("X-Client-Name", clientName)
		}
		r.Header.Set("X-Is-Admin", fmt.Sprintf("%t", isAdmin))

		next.ServeHTTP(w, r)
	})
}

// sendUnauthorizedResponse sends a standardized unauthorized response
func (s *Server) sendUnauthorizedResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// getClientAccessFromRequest extracts client access information from request headers
func getClientAccessFromRequest(r *http.Request) (clientName string, isAdmin bool) {
	isAdminStr := r.Header.Get("X-Is-Admin")
	isAdmin = isAdminStr == "true"

	if !isAdmin {
		clientName = r.Header.Get("X-Client-Name")
	}

	return clientName, isAdmin
}

// extractAPIKey extracts API key from request headers or query parameters
func (s *Server) extractAPIKey(r *http.Request) string {
	// Check Authorization header (Bearer token)
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}

	// Check X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}

	// Check query parameter
	if apiKey := r.URL.Query().Get("apikey"); apiKey != "" {
		// URL decode the parameter
		if decoded, err := url.QueryUnescape(apiKey); err == nil {
			return decoded
		}
		return apiKey
	}

	return ""
}

// parseAPIKey parses an API key to determine its type and extract components
// Returns: clientName, clientAuth, isAdmin
// For admin keys (format: "clientAuth"): "", clientAuth, true
// For client keys (format: "clientName-clientAuth"): clientName, clientAuth, false
//
// Logic: If the key contains exactly one hyphen and both parts are non-empty,
// treat it as a client key. Otherwise, treat it as an admin key.
func parseAPIKey(apiKey string) (clientName string, clientAuth string, isAdmin bool) {
	parts := strings.Split(apiKey, "-")

	// If exactly 2 parts and both are non-empty, treat as client key
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], false // Standard client key
	}

	// Otherwise, treat as admin key (including keys with multiple hyphens)
	return "", apiKey, true // Admin key
}

// validateAPIKeyAccess validates API key access based on type
func (s *Server) validateAPIKeyAccess(clientName, clientAuth string, isAdmin bool) bool {
	if isAdmin {
		// For admin keys, check if clientAuth exists in API_KEYS
		return s.isValidAPIKey(clientAuth)
	}
	// For client keys, check if clientName-clientAuth combination exists
	fullKey := clientName + "-" + clientAuth
	return s.isValidAPIKey(fullKey)
}

// isValidAPIKey checks if the provided API key is valid using constant-time comparison
func (s *Server) isValidAPIKey(providedKey string) bool {
	for _, validKey := range s.apiKeys {
		if len(providedKey) == len(validKey) &&
			subtle.ConstantTimeCompare([]byte(providedKey), []byte(validKey)) == 1 {
			return true
		}
	}
	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
