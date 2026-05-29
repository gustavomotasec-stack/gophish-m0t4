package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserId int64  `json:"user_id"`
	ApiKey string `json:"api_key"`
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "changeme-set-JWT_SECRET-env-var"
	}
	return []byte(s)
}

// GenerateToken creates a signed JWT for the user
func GenerateToken(userId int64, apiKey string) (string, error) {
	claims := &Claims{
		UserId: userId,
		ApiKey: apiKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret())
}

// ValidateRequest extracts and validates the JWT from the request.
// Checks: Authorization: Bearer <token>, cookie "gophish_token", query ?api_key=
func ValidateRequest(r *http.Request) (*Claims, error) {
	tokenStr := ""

	// 1. Authorization header
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	}
	// 2. Cookie
	if tokenStr == "" {
		if c, err := r.Cookie("gophish_token"); err == nil {
			tokenStr = c.Value
		}
	}
	// 3. Query param (for API key compat)
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("api_key")
	}

	if tokenStr == "" {
		return nil, jwt.ErrTokenMalformed
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret(), nil
	})
	return claims, err
}

// RequireAuth wraps a handler and returns 401 if not authenticated
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if _, err := ValidateRequest(r); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
			return
		}
		next(w, r)
	}
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
}

// JSON helper
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func ErrorJSON(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"message": msg})
}
