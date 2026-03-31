package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// User represents an authenticated user in the system.
type User struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // never returned in API responses
	Role     string `json:"role"`               // "admin" or "viewer"
	Token    string `json:"token,omitempty"`    // session token
}

// UserStore manages users and session tokens in memory.
type UserStore struct {
	mu     sync.RWMutex
	users  map[string]*User // username -> user
	tokens map[string]*User // token -> user
}

// NewUserStore creates a UserStore with a default admin user (admin/admin).
func NewUserStore() *UserStore {
	store := &UserStore{
		users:  make(map[string]*User),
		tokens: make(map[string]*User),
	}
	// Create default admin user.
	store.CreateUser("admin", "admin", "admin")
	return store
}

// CreateUser adds a new user with the given credentials and role.
func (s *UserStore) CreateUser(username, password, role string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[username] = &User{
		Username: username,
		Password: hashPassword(password),
		Role:     role,
	}
}

// Authenticate validates credentials and returns a session token on success.
func (s *UserStore) Authenticate(username, password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[username]
	if !ok || user.Password != hashPassword(password) {
		return "", fmt.Errorf("invalid credentials")
	}
	token := generateToken()
	s.tokens[token] = user
	return token, nil
}

// ValidateToken returns the user associated with the given token, or nil.
func (s *UserStore) ValidateToken(token string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokens[token]
}

func hashPassword(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:])
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// login handles POST /api/v1/auth/login.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	token, err := s.users.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":    token,
		"username": req.Username,
	})
}

// me handles GET /api/v1/auth/me.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing Authorization header"})
		return
	}

	key := strings.TrimPrefix(auth, "Bearer ")
	user := s.users.ValidateToken(key)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"username": user.Username,
		"role":     user.Role,
	})
}
