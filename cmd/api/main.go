package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/vricosti/hibana-stack/internal/api"
	"github.com/vricosti/hibana-stack/internal/config"
	"github.com/vricosti/hibana-stack/internal/installer"
)

func main() {
	// Parse command line flags
	port := flag.String("port", "3000", "Port to listen on")
	staticDir := flag.String("static", "./web/admin", "Directory for static files")
	configPath := flag.String("config", "/etc/hibana/hibana-config.yaml", "Path to hibana config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		// Config might not exist, use defaults
		log.Printf("Warning: Could not load config: %v", err)
		cfg = &config.Config{}
	}

	// Load database configuration
	hibanaConnStr, pdnsConnStr, err := api.LoadDBConfig()
	if err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}

	// Connect to databases
	hibanaDB, err := sql.Open("postgres", hibanaConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to Hibana database: %v", err)
	}
	defer hibanaDB.Close()

	if err := hibanaDB.Ping(); err != nil {
		log.Fatalf("Failed to ping Hibana database: %v", err)
	}
	log.Println("Connected to Hibana database")

	pdnsDB, err := sql.Open("postgres", pdnsConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to PowerDNS database: %v", err)
	}
	defer pdnsDB.Close()

	if err := pdnsDB.Ping(); err != nil {
		log.Fatalf("Failed to ping PowerDNS database: %v", err)
	}
	log.Println("Connected to PowerDNS database")

	// Create installer instance for domain user management
	inst := installer.NewInstaller(cfg, hibanaDB)

	// Get JWT secret
	jwtSecret := getJWTSecret()

	// Create and start server
	server := api.NewServer(hibanaDB, pdnsDB, inst, jwtSecret, *port, *staticDir)

	log.Fatal(server.Start())
}

// getJWTSecret gets the JWT secret from environment or file
func getJWTSecret() string {
	// Try to get from environment first
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}

	// Check for JWT_SECRET_FILE environment variable (Docker secrets)
	if secretFile := os.Getenv("JWT_SECRET_FILE"); secretFile != "" {
		data, err := os.ReadFile(secretFile)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
		log.Printf("Warning: Could not read JWT_SECRET_FILE %s: %v", secretFile, err)
	}

	// Try Docker secrets path (inside container)
	dockerSecretPath := "/run/secrets/jwt_secret"
	data, err := os.ReadFile(dockerSecretPath)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// Try local secrets path
	secretPath := "/etc/hibana/secrets/jwt_secret"
	data, err = os.ReadFile(secretPath)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// Generate and save new secret
	secret, err := generateAndSaveJWTSecret(secretPath)
	if err != nil {
		log.Printf("Warning: Could not generate JWT secret: %v", err)
		return "default-insecure-secret-change-this"
	}

	return secret
}

// generateAndSaveJWTSecret generates a new JWT secret and saves it
func generateAndSaveJWTSecret(path string) (string, error) {
	// Generate random secret (in production, use crypto/rand)
	secret := fmt.Sprintf("hibana-jwt-secret-%d", os.Getpid())

	// Create directory if it doesn't exist
	os.MkdirAll("/etc/hibana/secrets", 0700)

	// Write secret to file
	err := os.WriteFile(path, []byte(secret), 0600)
	if err != nil {
		return "", err
	}

	log.Printf("Generated new JWT secret at %s", path)
	return secret, nil
}
