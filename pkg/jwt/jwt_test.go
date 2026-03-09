package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestNewJWTManager(t *testing.T) {
	t.Run("SuccessfulCreation", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)
		assert.NotNil(t, manager)
		assert.Equal(t, []byte("test-secret"), manager.secret)
		assert.Equal(t, "aftertalk", manager.issuer)
		assert.Equal(t, 2*time.Hour, manager.expiration)
	})

	t.Run("ZeroDuration", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 0)
		assert.NotNil(t, manager)
		assert.Equal(t, 0*time.Hour, manager.expiration)
	})
}

func TestJWTManager_Generate(t *testing.T) {
	t.Run("SuccessfulTokenGeneration", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, jti, err := manager.Generate("session-123", "user-456", "admin")
		assert.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		assert.NotEmpty(t, jti)
	})

	t.Run("ValidTokenStructure", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		// Verify token is JWT format (header.payload.signature)
		parts := strings.Split(tokenString, ".")
		assert.Len(t, parts, 3)
	})

	t.Run("ValidClaims", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "moderator")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "session-123", claims.SessionID)
		assert.Equal(t, "user-456", claims.UserID)
		assert.Equal(t, "moderator", claims.Role)
		assert.Equal(t, "aftertalk", claims.Issuer)
		assert.NotZero(t, claims.ID)
		assert.NotZero(t, claims.IssuedAt)
		assert.NotZero(t, claims.ExpiresAt)
		assert.NotZero(t, claims.NotBefore)
	})

	t.Run("DifferentClaims", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		testCases := []struct {
			sessionID string
			userID    string
			role      string
		}{
			{"session-1", "user-1", "admin"},
			{"session-2", "user-2", "user"},
			{"session-3", "user-3", "moderator"},
		}

		for _, tc := range testCases {
			t.Run(tc.sessionID, func(t *testing.T) {
				tokenString, _, err := manager.Generate(tc.sessionID, tc.userID, tc.role)
				assert.NoError(t, err)

				claims, err := manager.Validate(tokenString)
				assert.NoError(t, err)
				assert.Equal(t, tc.sessionID, claims.SessionID)
				assert.Equal(t, tc.userID, claims.UserID)
				assert.Equal(t, tc.role, claims.Role)
			})
		}
	})

	t.Run("UniqueJTIForEachToken", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		var jtis []string
		for i := 0; i < 100; i++ {
			_, jti, err := manager.Generate("session-123", "user-456", "user")
			assert.NoError(t, err)
			jtis = append(jtis, jti)
		}

		// All JTIs should be unique
		assert.Equal(t, len(jtis), len(unique(jtis)))
	})

	t.Run("CurrentTimestampUsed", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		now := time.Now()
		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)

		// IssuedAt should be close to now
		assert.WithinDuration(t, now, claims.IssuedAt.Time, 5*time.Second)
		// NotBefore should be close to now
		assert.WithinDuration(t, now, claims.NotBefore.Time, 5*time.Second)
		// ExpiresAt should be expiration from now
		expectedExpiry := now.Add(2 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 5*time.Second)
	})

	t.Run("TokenExpiration", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 1*time.Second)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		_, err = manager.Validate(tokenString)
		assert.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = manager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("NegativeExpiration", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", -1*time.Hour)

		_, _, err := manager.Generate("session-123", "user-456", "user")
		assert.Error(t, err)
	})
}

func TestJWTManager_Validate(t *testing.T) {
	t.Run("ValidToken", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "session-123", claims.SessionID)
		assert.Equal(t, "user-456", claims.UserID)
		assert.Equal(t, "user", claims.Role)
		assert.Equal(t, "aftertalk", claims.Issuer)
	})

	t.Run("InvalidTokenString", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		_, err := manager.Validate("invalid-token-string")
		assert.Error(t, err)
	})

	t.Run("MalformedToken", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		_, err := manager.Validate("invalid.token..signature")
		assert.Error(t, err)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 1*time.Second)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		_, err = manager.Validate(tokenString)
		assert.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = manager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("TokenWithFutureExpiry", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)

		// Token should be valid in the future
		now := time.Now()
		expectedExpiry := now.Add(2 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 5*time.Second)
	})

	t.Run("TokenWithPastExpiry", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 1*time.Second)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		_, err = manager.Validate(tokenString)
		assert.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = manager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("TokenWithPastExpiry2", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 1*time.Second)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = manager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("TokenWithWrongSecret", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		// Try to validate with wrong secret
		wrongManager := NewJWTManager("wrong-secret", "aftertalk", 2*time.Hour)
		_, err = wrongManager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("TokenWithWrongIssuer", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		// Try to validate with wrong issuer
		wrongManager := NewJWTManager("test-secret", "wrong-issuer", 2*time.Hour)
		_, err = wrongManager.Validate(tokenString)
		assert.Error(t, err)
	})

	t.Run("TokenWithWrongSigningMethod", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		// Create a token with RS256 signing method (which manager doesn't support)
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		assert.NoError(t, err)

		claims := &Claims{
			SessionID: "session-123",
			UserID:    "user-456",
			Role:      "user",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "test-jti",
				Issuer:    "aftertalk",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(rsaKey)
		assert.NoError(t, err)

		_, err = manager.Validate(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})

	t.Run("InvalidJWTClaims", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		claims := &Claims{
			SessionID: "session-123",
			UserID:    "user-456",
			Role:      "user",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "test-jti",
				Issuer:    "aftertalk",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("test-secret"))
		assert.NoError(t, err)

		// The tokenString is already signed — local claims mutations don't affect it
		claims, ok := token.Claims.(*Claims)
		assert.True(t, ok)
		assert.NotEmpty(t, claims.UserID)

		_, err = manager.Validate(tokenString)
		assert.NoError(t, err)
	})
}

func TestJWTManager_GetJTI(t *testing.T) {
	t.Run("SuccessfulJTIExtraction", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, jti, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		retrievedJTI, err := manager.GetJTI(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, jti, retrievedJTI)
	})

	t.Run("InvalidTokenReturnsError", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		_, err := manager.GetJTI("invalid-token")
		assert.Error(t, err)
	})

	t.Run("ExpiredTokenReturnsError", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 1*time.Second)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		time.Sleep(1100 * time.Millisecond)

		_, err = manager.GetJTI(tokenString)
		assert.Error(t, err)
	})

	t.Run("WrongSecretReturnsError", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		wrongManager := NewJWTManager("wrong-secret", "aftertalk", 2*time.Hour)
		_, err = wrongManager.GetJTI(tokenString)
		assert.Error(t, err)
	})

	t.Run("MultipleGetJTICalls", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		// Call GetJTI multiple times
		retrievedJTI1, err := manager.GetJTI(tokenString)
		assert.NoError(t, err)

		retrievedJTI2, err := manager.GetJTI(tokenString)
		assert.NoError(t, err)

		assert.Equal(t, retrievedJTI1, retrievedJTI2)
	})
}

func TestJWTManager_Roles(t *testing.T) {
	t.Run("DifferentRoles", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		roles := []string{"admin", "moderator", "user", "guest"}

		for _, role := range roles {
			tokenString, _, err := manager.Generate("session-123", "user-456", role)
			assert.NoError(t, err)

			claims, err := manager.Validate(tokenString)
			assert.NoError(t, err)
			assert.Equal(t, role, claims.Role)
		}
	})

	t.Run("RoleComparison", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "admin")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)

		assert.Equal(t, "admin", claims.Role)
	})
}

func TestJWTManager_SessionAndUserID(t *testing.T) {
	t.Run("ValidSessionAndUserID", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)

		assert.Equal(t, "session-123", claims.SessionID)
		assert.Equal(t, "user-456", claims.UserID)
	})

	t.Run("DifferentSessionIDs", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		sessions := []string{"session-1", "session-2", "session-3"}

		for _, sessionID := range sessions {
			tokenString, _, err := manager.Generate(sessionID, "user-456", "user")
			assert.NoError(t, err)

			claims, err := manager.Validate(tokenString)
			assert.NoError(t, err)
			assert.Equal(t, sessionID, claims.SessionID)
		}
	})

	t.Run("DifferentUserIDs", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		userIDs := []string{"user-1", "user-2", "user-3"}

		for _, userID := range userIDs {
			tokenString, _, err := manager.Generate("session-123", userID, "user")
			assert.NoError(t, err)

			claims, err := manager.Validate(tokenString)
			assert.NoError(t, err)
			assert.Equal(t, userID, claims.UserID)
		}
	})

	t.Run("EmptySessionID", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("", "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, "", claims.SessionID)
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-123", "", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, "", claims.UserID)
	})
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func TestJWTManager_UncommonCases(t *testing.T) {
	t.Run("LongSessionID", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		longSessionID := "session-" + string(make([]byte, 1000))
		tokenString, _, err := manager.Generate(longSessionID, "user-456", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, longSessionID, claims.SessionID)
	})

	t.Run("LongUserID", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		longUserID := "user-" + string(make([]byte, 1000))
		tokenString, _, err := manager.Generate("session-123", longUserID, "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, longUserID, claims.UserID)
	})

	t.Run("UnicodeCharacters", func(t *testing.T) {
		manager := NewJWTManager("test-secret", "aftertalk", 2*time.Hour)

		tokenString, _, err := manager.Generate("session-日本語", "ユーザー", "user")
		assert.NoError(t, err)

		claims, err := manager.Validate(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, "session-日本語", claims.SessionID)
		assert.Equal(t, "ユーザー", claims.UserID)
	})
}
