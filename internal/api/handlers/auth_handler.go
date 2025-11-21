package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/vricosti/hibana-stack/internal/api/auth"
	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/ratelimit"
	"golang.org/x/crypto/bcrypt"
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

	// Validate credentials against email accounts table
	// For simplicity, we're using email accounts as admin users
	// In production, you'd have a separate admin_users table
	var userID int
	var passwordHash string
	query := `SELECT ea.id, ea.password_hash
	          FROM email_accounts ea
	          JOIN domains d ON ea.domain_id = d.id
	          WHERE ea.username = $1
	          ORDER BY d.created_at ASC LIMIT 1`

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

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
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
