package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"krelease-tracker/internal/api"
	"krelease-tracker/internal/config"
	"krelease-tracker/internal/database"
	"krelease-tracker/internal/kubernetes"
	"krelease-tracker/internal/ping"
	"krelease-tracker/internal/sync"
)

func main() {
	log.Println("Starting Release Tracker...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Configuration loaded: Port=%s, DatabasePath=%s, Namespaces=%v, Mode=%s",
		cfg.Port, cfg.DatabasePath, cfg.Namespaces, cfg.Mode)

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized")

	// Initialize Kubernetes client
	k8s, err := kubernetes.New(cfg.InCluster, cfg.KubeconfigPath, cfg.Namespaces, cfg.Mode)
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}
	log.Println("Kubernetes client initialized")

	// Initialize API server
	apiServer := api.New(db, k8s, cfg)
	log.Println("API server initialized")

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      apiServer,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start periodic collection in background (only in slave mode)
	if cfg.Mode == "slave" {
		log.Println("Starting periodic collection (slave mode)")
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.CollectionInterval) * time.Minute)
			defer ticker.Stop()

			// Initial collection and sync
			log.Println("Performing initial collection...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := k8s.CollectReleases(ctx, db); err != nil {
				log.Printf("Initial collection failed: %v", err)
			} else {
				log.Println("Initial collection completed")
				// Force first sync after initial collection
				syncClient := sync.New(cfg.MasterURL, cfg.MasterAPIKey, db, cfg.ProxyURL, cfg.TLSInsecure)
				if err := syncClient.SyncPendingReleases(ctx); err != nil {
					log.Printf("Initial sync failed: %v", err)
				} else {
					log.Println("Initial sync completed")
				}
			}
			cancel()

			// Periodic collection
			for {
				select {
				case <-ticker.C:
					log.Println("Starting periodic collection...")
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					if err := k8s.CollectReleases(ctx, db); err != nil {
						log.Printf("Periodic collection failed: %v", err)
					} else {
						log.Println("Periodic collection completed")
					}
					cancel()
				}
			}
		}()
	} else {
		log.Println("Periodic collection disabled (master mode)")
	}

	// Start sync worker in slave mode
	if cfg.Mode == "slave" && cfg.MasterURL != "" {
		log.Printf("Starting sync worker (slave mode) - Master URL: %s, Sync Interval: %d minutes", cfg.MasterURL, cfg.SyncInterval)

		syncClient := sync.New(cfg.MasterURL, cfg.MasterAPIKey, db, cfg.ProxyURL, cfg.TLSInsecure)
		go syncClient.StartSyncWorker(context.Background(), time.Duration(cfg.SyncInterval)*time.Minute)

		// Start ping worker for health monitoring
		log.Printf("Starting ping worker (slave mode) - Ping Interval: 5 minutes")
		pingClient := ping.New(cfg.MasterURL, cfg.MasterAPIKey, cfg.ClientName, cfg.EnvName, "v1.0.0", cfg.ProxyURL, cfg.TLSInsecure)
		go pingClient.StartPingWorker(context.Background(), 5*time.Minute)
	} else if cfg.Mode == "slave" {
		log.Println("Sync worker disabled - MASTER_URL not configured")
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	log.Println("Server exited")
}
