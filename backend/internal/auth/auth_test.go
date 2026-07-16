package auth

import (
	"testing"

	"github.com/skydashnet/miniacs/internal/models"
)

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
}
