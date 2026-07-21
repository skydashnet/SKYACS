package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skydashnet/skyacs/internal/models"
)

type userLookupStub struct {
	user *models.User
}

func (lookup userLookupStub) GetByID(context.Context, int64) (*models.User, error) {
	return lookup.user, nil
}

func TestTokenRoundTrip(t *testing.T) {
	if err := ConfigureJWT("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	token, err := GenerateToken(&models.User{ID: 42, Username: "operator", Role: models.RoleRead})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 42 || claims.Username != "operator" || claims.Role != models.RoleRead {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if _, err := ValidateToken(token + "tampered"); err == nil {
		t.Fatal("tampered token was accepted")
	}
}

func TestConfigureJWTRejectsWeakSecret(t *testing.T) {
	if err := ConfigureJWT("too-short"); err == nil {
		t.Fatal("weak secret was accepted")
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, password := range []string{"short", "alllowercase123", "ALLUPPERCASE123", "NoNumbersHere"} {
		if err := ValidatePassword(password); err == nil {
			t.Fatalf("weak password %q was accepted", password)
		}
	}
	if err := ValidatePassword("ControlPlane2026"); err != nil {
		t.Fatalf("strong password rejected: %v", err)
	}
	if err := ValidatePassword("Uppercase123" + string(make([]byte, 72))); err == nil {
		t.Fatal("password exceeding bcrypt's input limit was accepted")
	}
}

func TestAuthMiddlewareRejectsRevokedToken(t *testing.T) {
	if err := ConfigureJWT("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	issued := &models.User{ID: 7, Username: "admin", Role: models.RoleFull, TokenVersion: 1}
	token, err := GenerateToken(issued)
	if err != nil {
		t.Fatal(err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	AuthMiddleware(userLookupStub{user: &models.User{ID: 7, Username: "admin", Role: models.RoleFull, TokenVersion: 2}}, next).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token returned status %d", recorder.Code)
	}
}
