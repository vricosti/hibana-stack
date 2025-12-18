package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vricosti/hibana-stack/internal/api/auth"
	"github.com/vricosti/hibana-stack/internal/api/handlers"
	"github.com/vricosti/hibana-stack/internal/api/ratelimit"
	"github.com/vricosti/hibana-stack/internal/api/services"
	"github.com/vricosti/hibana-stack/internal/installer"
)

// Server represents the API server
type Server struct {
	db           *sql.DB
	pdnsDB       *sql.DB
	installer    *installer.Installer
	jwtManager   *auth.JWTManager
	port         string
	staticDir    string
}

// NewServer creates a new API server
func NewServer(db, pdnsDB *sql.DB, inst *installer.Installer, jwtSecret, port, staticDir string) *Server {
	return &Server{
		db:         db,
		pdnsDB:     pdnsDB,
		installer:  inst,
		jwtManager: auth.NewJWTManager(jwtSecret, 24*time.Hour),
		port:       port,
		staticDir:  staticDir,
	}
}

// Start starts the API server
func (s *Server) Start() error {
	// Create rate limiter for login attempts
	// Block after 5 failed attempts for 15 minutes
	loginLimiter := ratelimit.NewLoginLimiter(5, 15*time.Minute)

	// Create services
	domainService := services.NewDomainService(s.db, s.pdnsDB, s.installer)
	emailService := services.NewEmailService(s.db)
	dnsService := services.NewDNSService(s.db, s.pdnsDB)
	sshKeyService := services.NewSSHKeyService(s.db)
	serviceService := services.NewServiceService(s.db)
	aliasService := services.NewAliasService(s.db, domainService)

	// Create handlers
	authHandler := handlers.NewAuthHandler(s.db, s.jwtManager, loginLimiter)
	domainHandler := handlers.NewDomainHandler(domainService)
	emailHandler := handlers.NewEmailHandler(emailService)
	dnsHandler := handlers.NewDNSHandler(dnsService)
	sshKeyHandler := handlers.NewSSHKeyHandler(sshKeyService)
	serviceHandler := handlers.NewServiceHandler(serviceService)
	aliasHandler := handlers.NewAliasHandler(aliasService)

	// Create router
	mux := http.NewServeMux()

	// Auth routes (no authentication required)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)

	// Health check
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// API routes (authentication required)
	authMiddleware := auth.AuthMiddleware(s.jwtManager)

	// Domain routes
	mux.Handle("/api/v1/domains", authMiddleware(http.HandlerFunc(domainHandler.HandleDomains)))
	mux.Handle("/api/v1/domains/", authMiddleware(http.HandlerFunc(s.routeDomainRequests(domainHandler, emailHandler, dnsHandler, sshKeyHandler, serviceHandler, aliasHandler))))
	mux.Handle("/api/v1/stats", authMiddleware(http.HandlerFunc(domainHandler.GetStats)))

	// Email routes
	mux.Handle("/api/v1/emails", authMiddleware(http.HandlerFunc(emailHandler.ListAll)))
	mux.Handle("/api/v1/emails/", authMiddleware(http.HandlerFunc(emailHandler.HandleEmail)))

	// Alias routes
	mux.Handle("/api/v1/aliases", authMiddleware(http.HandlerFunc(aliasHandler.ListAll)))
	mux.Handle("/api/v1/aliases/", authMiddleware(http.HandlerFunc(aliasHandler.HandleAlias)))

	// DNS routes
	mux.Handle("/api/v1/dns/", authMiddleware(http.HandlerFunc(dnsHandler.HandleDNSRecord)))

	// SSH Key generation
	mux.Handle("/api/v1/ssh-keys/generate", authMiddleware(http.HandlerFunc(sshKeyHandler.GenerateKeyPair)))

	// Static files for admin interface
	if s.staticDir != "" {
		fs := http.FileServer(http.Dir(s.staticDir))
		mux.Handle("/", s.staticFileHandler(fs))
	}

	// CORS middleware
	handler := s.corsMiddleware(mux)

	// Logging middleware
	handler = s.loggingMiddleware(handler)

	log.Printf("Starting Hibana API Server on port %s", s.port)
	log.Printf("API endpoints available at: http://localhost:%s/api/v1", s.port)
	if s.staticDir != "" {
		log.Printf("Admin interface available at: http://localhost:%s", s.port)
	}

	return http.ListenAndServe(":"+s.port, handler)
}

// routeDomainRequests routes requests starting with /api/v1/domains/:id
func (s *Server) routeDomainRequests(domainHandler *handlers.DomainHandler, emailHandler *handlers.EmailHandler, dnsHandler *handlers.DNSHandler, sshKeyHandler *handlers.SSHKeyHandler, serviceHandler *handlers.ServiceHandler, aliasHandler *handlers.AliasHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Route based on the path structure
		if strings.Contains(path, "/services") {
			serviceHandler.HandleServices(w, r)
		} else if strings.Contains(path, "/ssh-keys") {
			sshKeyHandler.HandleDomainSSHKeys(w, r)
		} else if strings.Contains(path, "/emails") {
			emailHandler.HandleDomainEmails(w, r)
		} else if strings.Contains(path, "/aliases") {
			aliasHandler.HandleDomainAliases(w, r)
		} else if strings.Contains(path, "/dns") {
			dnsHandler.HandleDomainDNS(w, r)
		} else {
			domainHandler.HandleDomain(w, r)
		}
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"hibana-api","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// staticFileHandler serves static files and handles SPA routing
func (s *Server) staticFileHandler(fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if file exists
		path := filepath.Join(s.staticDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File doesn't exist, serve index.html for SPA routing
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
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

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the next handler
		next.ServeHTTP(w, r)

		// Log the request
		duration := time.Since(start)
		log.Printf("%s %s - %v", r.Method, r.URL.Path, duration)
	})
}

// LoadDBConfig loads database configuration from environment variables and secrets
func LoadDBConfig() (hibanaConnStr, pdnsConnStr string, err error) {
	// Get database host from environment or use default
	dbHost := os.Getenv("DATABASE_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	// Get database name from environment or use default
	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "hibana"
	}

	// Get database user from environment or use default
	dbUser := os.Getenv("DATABASE_USER")
	if dbUser == "" {
		dbUser = "hibana"
	}

	// Read database password from Docker secrets or local file
	var passwordData []byte
	if passwordFile := os.Getenv("DATABASE_PASSWORD_FILE"); passwordFile != "" {
		// Try Docker secrets path from environment
		passwordData, err = os.ReadFile(passwordFile)
		if err != nil {
			return "", "", fmt.Errorf("failed to read password from %s: %w", passwordFile, err)
		}
	} else {
		// Try Docker secrets default path
		passwordData, err = os.ReadFile("/run/secrets/db_password")
		if err != nil {
			// Fall back to local path
			passwordData, err = os.ReadFile("/etc/hibana/secrets/postgresql_hibana_password")
			if err != nil {
				return "", "", fmt.Errorf("failed to read hibana password: %w", err)
			}
		}
	}

	hibanaPassword := strings.TrimSpace(string(passwordData))

	// Read PDNS password from separate secret
	var pdnsPasswordData []byte
	if pdnsPasswordFile := os.Getenv("PDNS_PASSWORD_FILE"); pdnsPasswordFile != "" {
		// Try Docker secrets path from environment
		pdnsPasswordData, err = os.ReadFile(pdnsPasswordFile)
		if err != nil {
			return "", "", fmt.Errorf("failed to read PDNS password from %s: %w", pdnsPasswordFile, err)
		}
	} else {
		// Try Docker secrets default path
		pdnsPasswordData, err = os.ReadFile("/run/secrets/pdns_password")
		if err != nil {
			// Fall back to local path
			pdnsPasswordData, err = os.ReadFile("/etc/hibana/passwords/pdns")
			if err != nil {
				return "", "", fmt.Errorf("failed to read pdns password: %w", err)
			}
		}
	}

	pdnsPassword := strings.TrimSpace(string(pdnsPasswordData))

	hibanaConnStr = fmt.Sprintf("host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, hibanaPassword, dbName)

	pdnsConnStr = fmt.Sprintf("host=%s port=5432 user=pdns password=%s dbname=powerdns sslmode=disable",
		dbHost, pdnsPassword)

	return hibanaConnStr, pdnsConnStr, nil
}
