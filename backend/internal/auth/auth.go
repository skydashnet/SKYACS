package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/skydashnet/miniacs/internal/models"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type contextKey string

const UserContextKey contextKey = "user"

const (
	jwtIssuer         = "miniacs"
	jwtAudience       = "miniacs-web"
	minSecretLength   = 32
	minPasswordLength = 12
	maxPasswordLength = 72
)

var jwtSecret []byte

type Claims struct {
	UserID       int64           `json:"user_id"`
	Username     string          `json:"username"`
	Role         models.UserRole `json:"role"`
	TokenVersion uint64          `json:"token_version"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateToken(user *models.User) (string, error) {
	claims := &Claims{
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-30 * time.Second)),
			Subject:   user.Username,
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}

	if len(jwtSecret) < minSecretLength {
		return "", errors.New("JWT secret is not configured")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateToken(tokenString string) (*Claims, error) {
	if len(jwtSecret) < minSecretLength {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return jwtSecret, nil
	}, jwt.WithIssuer(jwtIssuer), jwt.WithAudience(jwtAudience), jwt.WithLeeway(30*time.Second), jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

type UserLookup interface {
	GetByID(context.Context, int64) (*models.User, error)
}

func AuthMiddleware(users UserLookup, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAuthError(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		claims, err := ValidateToken(parts[1])
		if err != nil {
			writeAuthError(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		user, err := users.GetByID(r.Context(), claims.UserID)
		if err != nil || user == nil || user.Username != claims.Username || user.Role != claims.Role || user.TokenVersion != claims.TokenVersion {
			writeAuthError(w, `{"error":"session has been revoked"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter, body string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func GetUserFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

func RequireFullAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserFromContext(r.Context())
		if claims == nil {
			writeAuthError(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		if claims.Role != models.RoleFull {
			writeAuthError(w, `{"error":"forbidden - full access required"}`, http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func ConfigureJWT(secret string) error {
	if len(secret) < minSecretLength {
		return fmt.Errorf("JWT_SECRET must contain at least %d characters", minSecretLength)
	}
	jwtSecret = []byte(secret)
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		return fmt.Errorf("password must not exceed %d bytes", maxPasswordLength)
	}

	var hasUpper, hasLower, hasNumber bool
	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		}
	}
	if !hasUpper || !hasLower || !hasNumber {
		return errors.New("password must include uppercase, lowercase, and numeric characters")
	}
	return nil
}
