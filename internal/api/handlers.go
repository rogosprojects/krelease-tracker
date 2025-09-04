package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"release-tracker/internal/config"
	"release-tracker/internal/database"
	"release-tracker/internal/kubernetes"

	"github.com/gorilla/mux"
)

// Server holds the API server dependencies
type Server struct {
	db         *database.DB
	k8s        *kubernetes.Client
	router     *mux.Router
	namespaces []string
	apiKeys    []string
	envName    string
	config     *config.Config
}

// New creates a new API server
func New(db *database.DB, k8s *kubernetes.Client, cfg *config.Config) *Server {
	s := &Server{
		db:         db,
		k8s:        k8s,
		router:     mux.NewRouter(),
		namespaces: cfg.Namespaces,
		apiKeys:    cfg.APIKeys,
		envName:    cfg.EnvName,
		config:     cfg,
	}

	s.setupRoutes()
	return s
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// handleCollect triggers collection of current cluster state asynchronously
func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	log.Printf("Collection triggered via API")

	// Start the collection process in the background
	go s.runCollectionAsync()

	// Immediately return acknowledgment response
	response := map[string]interface{}{
		"status":    "accepted",
		"message":   "Collection process started successfully",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// runCollectionAsync runs the collection process in the background
func (s *Server) runCollectionAsync() {
	// Create a background context with timeout for the collection process
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("Starting background collection process")

	// Check if kubernetes client is available
	if s.k8s == nil {
		log.Printf("Background collection skipped: kubernetes client not available")
		return
	}

	if err := s.k8s.CollectReleases(ctx, s.db); err != nil {
		log.Printf("Background collection failed: %v", err)
		return
	}

	log.Printf("Background collection completed successfully")
}

// ManualCollectRequest represents the request body for manual collection
type ManualCollectRequest struct {
	ImageTag   string     `json:"image_tag,omitempty"`
	ImageSHA   string     `json:"image_sha,omitempty"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	ImageRepo  string     `json:"image_repo,omitempty"`
	ImageName  string     `json:"image_name,omitempty"`
	ClientName string     `json:"client_name,omitempty"`
	EnvName    string     `json:"env_name,omitempty"`
}

// handleManualCollect manually adds a new workload release to the database
func (s *Server) handleManualCollect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	workloadKind := vars["workload-kind"]
	workloadName := vars["workload-name"]
	container := vars["container"]

	// Validate path parameters
	if namespace == "" || workloadKind == "" || workloadName == "" || container == "" {
		http.Error(w, "Missing required path parameters: namespace, workload-kind, workload-name, container", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req ManualCollectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ImageTag == "" || req.ImageSHA == "" {
		http.Error(w, "Missing required field: image_tag, image_sha", http.StatusBadRequest)
		return
	}

	// Default released_at to now if not provided
	releasedAt := time.Now().UTC()
	if req.ReleasedAt != nil {
		releasedAt = req.ReleasedAt.UTC()
	}

	imagePath := fmt.Sprintf("%s/%s:%s", req.ImageRepo, req.ImageName, req.ImageTag)

	// Parse the release version (image path) into components
	repo, name, tag := database.ParseImagePath(imagePath)

	// Get client and environment names from request or environment variables
	clientName := req.ClientName
	if clientName == "" {
		clientName = s.config.ClientName
	}
	envName := req.EnvName
	if envName == "" {
		envName = s.config.EnvName
	}

	// Create release object
	release := &database.Release{
		Namespace:     namespace,
		WorkloadName:  workloadName,
		WorkloadType:  workloadKind,
		ContainerName: container,
		ImageRepo:     repo,
		ImageName:     name,
		ImageTag:      tag,
		ImageSHA:      req.ImageSHA,
		ClientName:    clientName,
		EnvName:       envName,
		FirstSeen:     releasedAt,
		LastSeen:      releasedAt,
	}

	// Save to database
	if err := s.db.UpsertRelease(release); err != nil {
		log.Printf("Failed to save manual release for %s/%s/%s/%s: %v", namespace, workloadKind, workloadName, container, err)
		http.Error(w, fmt.Sprintf("Failed to save release: %v", err), http.StatusInternalServerError)
		return
	}

	if s.config.Mode == "slave" {
		// In slave mode, also store in pending_releases table as queue
		pendingRelease := &database.PendingRelease{
			Namespace:     namespace,
			WorkloadName:  workloadName,
			WorkloadType:  workloadKind,
			ContainerName: container,
			ImageRepo:     repo,
			ImageName:     name,
			ImageTag:      tag,
			ImageSHA:      req.ImageSHA,
			ClientName:    clientName,
			EnvName:       envName,
			FirstSeen:     releasedAt,
			LastSeen:      releasedAt,
		}

		if err := s.db.UpsertPendingRelease(pendingRelease); err != nil {
			log.Printf("Failed to upsert pending release for %s/%s/%s/%s: %v", namespace, workloadKind, workloadName, container, err)
			http.Error(w, fmt.Sprintf("Failed to upsert pending release: %v", err), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Manual release collected: %s at %s %s/%s/%s/%s -> %s", clientName, envName, namespace, workloadKind, workloadName, container, req.ImageTag)

	response := map[string]interface{}{
		"status":  "success",
		"message": "Release collected successfully",
		"component": map[string]string{
			"namespace":      namespace,
			"workload_kind":  workloadKind,
			"workload_name":  workloadName,
			"container_name": container,
		},
		"release": map[string]interface{}{
			"version":     req.ImageTag,
			"image_repo":  repo,
			"image_name":  name,
			"image_tag":   tag,
			"image_sha":   req.ImageSHA,
			"released_at": releasedAt,
		},
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCurrentReleases returns all current deployed images
func (s *Server) handleCurrentReleases(w http.ResponseWriter, r *http.Request) {
	// Get client_name and env_name filters from query parameters (required)
	requestedClientName := r.URL.Query().Get("client_name")
	envName := r.URL.Query().Get("env_name")

	if requestedClientName == "" || envName == "" {
		http.Error(w, "Missing required query parameters: client_name, env_name", http.StatusBadRequest)
		return
	}

	// Check client access permissions
	authenticatedClientName, isAdmin := getClientAccessFromRequest(r)
	if !isAdmin && authenticatedClientName != requestedClientName {
		log.Printf("Access denied for %s %s: API key not authorized for client '%s'", r.Method, r.URL.Path, requestedClientName)
		http.Error(w, fmt.Sprintf("Access denied: API key is not authorized for client '%s'", requestedClientName), http.StatusForbidden)
		return
	}

	releases, err := s.db.GetCurrentReleasesFiltered(requestedClientName, envName)
	if err != nil {
		log.Printf("Failed to get current releases: %v", err)
		http.Error(w, "Failed to get current releases", http.StatusInternalServerError)
		return
	}

	// Group releases by namespace for better organization
	grouped := make(map[string][]database.CurrentRelease)
	for _, release := range releases {
		grouped[release.Namespace] = append(grouped[release.Namespace], release)
	}

	// Create ordered namespace list based on configuration
	orderedNamespaces := make([]map[string]interface{}, 0)
	for _, namespace := range s.namespaces {
		if releases, exists := grouped[namespace]; exists {
			orderedNamespaces = append(orderedNamespaces, map[string]interface{}{
				"name":     namespace,
				"releases": releases,
			})
		}
	}

	// Add any namespaces not in configuration (in case of dynamic discovery)
	for namespace, releases := range grouped {
		found := false
		for _, configNs := range s.namespaces {
			if configNs == namespace {
				found = true
				break
			}
		}
		if !found {
			orderedNamespaces = append(orderedNamespaces, map[string]interface{}{
				"name":     namespace,
				"releases": releases,
			})
		}
	}

	lastUpdate, err := s.db.GetLastClientEnvUpdate(requestedClientName, envName)
	if err != nil {
		log.Printf("Failed to get last update: %v", err)
		http.Error(w, "Failed to get last update", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"namespaces":         grouped, // Keep for backward compatibility
		"ordered_namespaces": orderedNamespaces,
		"total":              len(releases),
		"timestamp":          lastUpdate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReleaseHistory returns release timeline for a specific component
func (s *Server) handleReleaseHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestedClientName := vars["client"]
	envName := vars["env"]
	namespace := vars["namespace"]
	workload := vars["workload"]
	container := vars["container"]

	if namespace == "" || workload == "" || container == "" || requestedClientName == "" || envName == "" {
		http.Error(w, "Missing required parameters: namespace, workload, container, client_name, env_name", http.StatusBadRequest)
		return
	}

	// Check client access permissions
	authenticatedClientName, isAdmin := getClientAccessFromRequest(r)
	if !isAdmin && authenticatedClientName != requestedClientName {
		log.Printf("Access denied for %s %s: API key not authorized for client '%s'", r.Method, r.URL.Path, requestedClientName)
		http.Error(w, fmt.Sprintf("Access denied: API key is not authorized for client '%s'", requestedClientName), http.StatusForbidden)
		return
	}

	history, err := s.db.GetReleaseHistory(namespace, workload, container, requestedClientName, envName)
	if err != nil {
		log.Printf("Failed to get release history for %s/%s/%s: %v", namespace, workload, container, err)
		http.Error(w, "Failed to get release history", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"component": map[string]string{
			"namespace":      namespace,
			"workload_name":  workload,
			"container_name": container,
		},
		"history":   history,
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns the health status of the application
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	}

	// Check database connectivity
	_, err := s.db.GetCurrentReleases()
	if err != nil {
		response["status"] = "unhealthy"
		response["database_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleBadgeWithAuth returns an SVG badge with URL-based API key authentication
func (s *Server) handleBadgeWithAuth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	apiKey := vars["api-key"]
	workloadKind := vars["workload-kind"]
	workloadName := vars["workload-name"]
	container := vars["container"]
	requestedClientName := vars["client"]
	envName := vars["env"]

	// Validate API key if authentication is enabled
	if len(s.apiKeys) > 0 {
		if apiKey == "" {
			log.Printf("Badge authentication failed for %s %s: missing API key", r.Method, r.URL.Path)
			badge := CreateErrorBadge(envName, "unauthorized")
			s.serveBadge(w, badge)
			return
		}

		// Parse API key to determine type and extract components
		authenticatedClientName, clientAuth, isAdmin := parseAPIKey(apiKey)

		// Validate API key access
		if !s.validateAPIKeyAccess(authenticatedClientName, clientAuth, isAdmin) {
			// Log failed authentication attempt with sanitized key
			keyPreview := apiKey[:min(8, len(apiKey))] + "..."
			log.Printf("Badge authentication failed for %s %s (key: %s)", r.Method, r.URL.Path, keyPreview)
			badge := CreateErrorBadge(envName, "unauthorized")
			s.serveBadge(w, badge)
			return
		}

		// Check client access permissions for standard API keys
		if !isAdmin && authenticatedClientName != requestedClientName {
			log.Printf("Badge access denied for %s %s: API key not authorized for client '%s'", r.Method, r.URL.Path, requestedClientName)
			badge := CreateErrorBadge(envName, "access denied")
			s.serveBadge(w, badge)
			return
		}
	}

	// Call the core badge logic
	s.handleBadgeCore(w, r, workloadKind, workloadName, container, requestedClientName, envName)
}

// handleBadgeCore contains the core badge generation logic
func (s *Server) handleBadgeCore(w http.ResponseWriter, r *http.Request, workloadKind, workloadName, container, clientName, envName string) {
	if workloadKind == "" || workloadName == "" || container == "" || clientName == "" || envName == "" {
		log.Printf("Badge request missing parameters: kind=%s, name=%s, container=%s, client=%s, env=%s", workloadKind, workloadName, container, clientName, envName)
		badge := CreateErrorBadge(envName, "invalid request")
		s.serveBadge(w, badge)
		return
	}

	// Query database for current release
	release, err := s.db.GetCurrentReleaseByWorkload(workloadKind, workloadName, container, clientName, envName)
	if err != nil {
		log.Printf("Badge query error for %s/%s/%s/%s/%s: %v", workloadKind, workloadName, container, clientName, envName, err)

		// Check if it's a "multiple found" error
		if strings.Contains(err.Error(), "multiple releases found") {
			badge := CreateMultipleFoundBadge(envName)
			s.serveBadge(w, badge)
			return
		}

		// Other database errors
		badge := CreateErrorBadge(envName, "query error")
		s.serveBadge(w, badge)
		return
	}

	if release == nil {
		// No release found
		log.Printf("No release found for %s/%s/%s/%s/%s", workloadKind, workloadName, container, clientName, envName)
		badge := CreateNotFoundBadge(envName)
		s.serveBadge(w, badge)
		return
	}

	// Success - create badge with version
	log.Printf("Badge generated for %s/%s/%s/%s/%s: %s", workloadKind, workloadName, container, clientName, envName, release.ImageTag)
	badge := CreateSuccessBadge(envName, release.ImageTag)
	s.serveBadge(w, badge)
}

// serveBadge sends the SVG badge with appropriate headers
func (s *Server) serveBadge(w http.ResponseWriter, svgContent string) {
	// Set headers for SVG content
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // Disable caching for real-time updates
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Enable CORS for embedding in README files
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Write SVG content
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(svgContent))
}

// handleClientsEnvironments returns available clients and environments for master mode
func (s *Server) handleClientsEnvironments(w http.ResponseWriter, r *http.Request) {
	// Check client access permissions
	authenticatedClientName, isAdmin := getClientAccessFromRequest(r)

	clientEnvs, err := s.db.GetAvailableClientsAndEnvironments()
	if err != nil {
		log.Printf("Failed to get clients and environments: %v", err)
		http.Error(w, "Failed to get clients and environments", http.StatusInternalServerError)
		return
	}

	// Filter clients based on access permissions
	if !isAdmin && authenticatedClientName != "" {
		// For standard API keys, only return the authenticated client
		if envs, exists := clientEnvs[authenticatedClientName]; exists {
			clientEnvs = map[string][]string{
				authenticatedClientName: envs,
			}
		} else {
			// Client not found, return empty result
			clientEnvs = make(map[string][]string)
		}
	}

	// Get ping status for accessible client/environment combinations
	pingStatuses := make(map[string]map[string]interface{})
	for clientName, envs := range clientEnvs {
		pingStatuses[clientName] = make(map[string]interface{})
		for _, envName := range envs {
			status, lastPing, err := s.db.GetSlavePingStatus(clientName, envName)
			if err != nil {
				log.Printf("Failed to get ping status for %s/%s: %v", clientName, envName, err)
				status = "unknown"
			}

			pingInfo := map[string]interface{}{
				"status": status,
			}
			if !lastPing.IsZero() {
				pingInfo["last_ping"] = lastPing.UTC()
			}

			pingStatuses[clientName][envName] = pingInfo
		}
	}

	// Calculate aggregate statistics
	totalClients := len(clientEnvs)
	totalEnvironments := 0
	for _, envs := range clientEnvs {
		totalEnvironments += len(envs)
	}

	allReleasesCount := 0

	if (isAdmin && authenticatedClientName == "") || (!isAdmin && authenticatedClientName != "") {
		// Get total releases count for all clients or just the authenticated client
		allReleases, err := s.db.GetCurrentReleasesFiltered(authenticatedClientName, "")
		if err != nil {
			log.Printf("Failed to get total releases count: %v", err)
			http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
			return
		}
		allReleasesCount = len(allReleases)
	}

	response := map[string]interface{}{
		"clients_environments": clientEnvs,
		"ping_statuses":        pingStatuses,
		"statistics": map[string]interface{}{
			"total_clients":      totalClients,
			"total_environments": totalEnvironments,
			"total_releases":     allReleasesCount,
		},
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// PingRequest represents the request body for slave ping
type PingRequest struct {
	ClientName   string `json:"client_name"`
	EnvName      string `json:"env_name"`
	SlaveVersion string `json:"slave_version,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
}

// handlePing receives health pings from slave instances
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	var req PingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ClientName == "" || req.EnvName == "" {
		http.Error(w, "client_name and env_name are required", http.StatusBadRequest)
		return
	}

	// Update ping record
	err := s.db.UpsertSlavePing(req.ClientName, req.EnvName, req.SlaveVersion)
	if err != nil {
		log.Printf("Failed to update slave ping for %s/%s: %v", req.ClientName, req.EnvName, err)
		http.Error(w, "Failed to update ping", http.StatusInternalServerError)
		return
	}

	log.Printf("Received ping from slave: %s/%s", req.ClientName, req.EnvName)

	// Return success response
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleConfig returns application configuration for the frontend
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	// Get client access information from authentication middleware
	authenticatedClientName, isAdmin := getClientAccessFromRequest(r)

	response := map[string]interface{}{
		"mode":        s.config.Mode,
		"env_name":    s.config.EnvName,
		"client_name": s.config.ClientName,
		"api_key_type": map[string]interface{}{
			"is_admin":             isAdmin,
			"authenticated_client": authenticatedClientName,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
