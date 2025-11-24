package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/vricosti/hibana-stack/internal/api/auth"
	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/ratelimit"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	db           *sql.DB
	jwtManager   *auth.JWTManager
	loginLimiter *ratelimit.LoginLimiter
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, jwtManager *auth.JWTManager, loginLimiter *ratelimit.LoginLimiter) *AuthHandler {
	return &AuthHandler{
		db:           db,
		jwtManager:   jwtManager,
		loginLimiter: loginLimiter,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string `json:"token"`
	User  UserInfo `json:"user"`
}

// UserInfo represents user information
type UserInfo struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get client IP
	clientIP := ratelimit.GetClientIP(
		r.RemoteAddr,
		r.Header.Get("X-Forwarded-For"),
		r.Header.Get("X-Real-IP"),
	)

	// Check if IP is blocked
	if blocked, _ := h.loginLimiter.IsBlocked(clientIP); blocked {
		log.Printf("Blocked login attempt from IP %s", clientIP)
		respondError(w, http.StatusTooManyRequests,
			"Too many failed login attempts. Try again later.")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate credentials against webadmin_users table
	var userID int
	var passwordHash string
	query := `SELECT id, password_hash FROM webadmin_users WHERE username = $1`

	err := h.db.QueryRow(query, req.Username).Scan(&userID, &passwordHash)
	if err == sql.ErrNoRows {
		// Record failed attempt
		h.loginLimiter.RecordFailedAttempt(clientIP)
		attemptCount := h.loginLimiter.GetAttemptCount(clientIP)
		log.Printf("Failed login attempt from IP %s for user %s (attempt %d)", clientIP, req.Username, attemptCount)

		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Verify password using SHA512 crypt (compatible with Ansible password_hash)
	if !verifySHA512Password(req.Password, passwordHash) {
		// Record failed attempt
		h.loginLimiter.RecordFailedAttempt(clientIP)
		attemptCount := h.loginLimiter.GetAttemptCount(clientIP)
		log.Printf("Failed login attempt from IP %s for user %s (attempt %d)", clientIP, req.Username, attemptCount)

		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Successful login - reset attempts
	h.loginLimiter.ResetAttempts(clientIP)
	log.Printf("Successful login from IP %s for user %s", clientIP, req.Username)

	// Generate JWT token
	token, err := h.jwtManager.Generate(userID, req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	response := LoginResponse{
		Token: token,
		User: UserInfo{
			ID:       userID,
			Username: req.Username,
		},
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    response,
	})
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// respondError sends a JSON error response
func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: false,
		Error:   message,
	})
}

// verifySHA512Password verifies a password against a SHA512 crypt hash
// This is compatible with Ansible's password_hash('sha512') filter
func verifySHA512Password(password, hash string) bool {
	// SHA512 crypt hashes start with $6$
	if !strings.HasPrefix(hash, "$6$") {
		return false
	}

	// Use the crypt library to verify
	crypter := crypt.SHA512.New()
	err := crypter.Verify(hash, []byte(password))
	return err == nil
}
