package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth/config"
	"auth/internal"
	"auth/middlewares"
	"auth/routes"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
)

func main() {
	// Create structured loggers
	infoLog := log.New(os.Stdout, "INFO  ", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)

	// 1. Load and validate configuration centrally
	cfg, err := config.Load()
	if err != nil {
		errorLog.Fatalf("Configuration failure: %v", err)
	}
	infoLog.Println("Configuration loaded and validated successfully")

	// 2. Initialize dedicated HTTP Client with explicit timeouts and connection pooling
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 3. Initialize Keycloak OIDC Provider (with retry logic to wait for Keycloak if still booting)
	discoveryURL := fmt.Sprintf("%s/realms/%s", cfg.KeycloakBaseURL, cfg.KeycloakRealm)
	var provider *oidc.Provider
	for attempt := 1; attempt <= 30; attempt++ {
		pCtx, pCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if cfg.KeycloakIssuerURL != "" && cfg.KeycloakIssuerURL != discoveryURL {
			pCtx = oidc.InsecureIssuerURLContext(pCtx, cfg.KeycloakIssuerURL)
		}
		provider, err = oidc.NewProvider(pCtx, discoveryURL)
		pCancel()
		if err == nil {
			break
		}
		infoLog.Printf("Waiting for Keycloak to finish starting up at %s (attempt %d/30)...", discoveryURL, attempt)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		errorLog.Fatalf("Failed to initialize OIDC provider after retries: %v", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
		SkipIssuerCheck:   true,
	})
	infoLog.Println("Keycloak OIDC initialized successfully")

	// 4. Construct Application dependency container
	app := &internal.Application{
		InfoLog:    infoLog,
		ErrorLog:   errorLog,
		Verifier:   verifier,
		Config:     cfg,
		HTTPClient: httpClient,
	}

	// 5. Setup Gin router and middleware
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	server := gin.Default()
	_ = server.SetTrustedProxies([]string{"127.0.0.1", "::1", "172.16.0.0/12", "192.168.0.0/16", "10.0.0.0/8"})

	// Health check endpoint for Docker, orchestrators, and load balancers
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":      "ok",
			"service":     "auth",
			"environment": cfg.Environment,
		})
	}
	server.GET("/health", healthHandler)
	server.HEAD("/health", healthHandler)

	// Apply CORS middleware globally for frontend integration
	server.Use(middlewares.CORS(cfg.AllowedOrigins))

	// Register route groups
	routes.AuthRoutes(server, app)

	// 6. Configure HTTP server with production timeouts
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a background goroutine
	go func() {
		infoLog.Printf("Server listening on port %s in %s mode\n", cfg.Port, cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorLog.Fatalf("Server stopped unexpectedly: %v", err)
		}
	}()

	// 7. Graceful Shutdown: Listen for interrupt signals (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	infoLog.Println("Shutdown signal received. Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		errorLog.Fatalf("Server forced to shutdown: %v", err)
	}

	infoLog.Println("Server exited cleanly. All active connections finished.")
}
