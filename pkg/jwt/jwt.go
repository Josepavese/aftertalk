package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	errJWTExpirationNotPositive = errors.New("invalid expiration: must be positive")
	errJWTUnexpectedSignMethod  = errors.New("unexpected signing method")
	errJWTInvalidClaims         = errors.New("invalid token claims")
)

type Claims struct {
	jwt.RegisteredClaims

	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
}

type JWTManager struct {
	issuer     string
	secret     []byte
	expiration time.Duration
}

func NewJWTManager(secret, issuer string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		issuer:     issuer,
		expiration: expiration,
	}
}

func (j *JWTManager) Generate(sessionID, userID, role string) (string, string, error) {
	if j.expiration <= 0 {
		return "", "", errJWTExpirationNotPositive
	}

	jti := uuid.New().String()

	now := time.Now()
	claims := &Claims{
		SessionID: sessionID,
		UserID:    userID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, nil
}

func (j *JWTManager) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", errJWTUnexpectedSignMethod, token.Header["alg"])
		}
		return j.secret, nil
	}, jwt.WithIssuer(j.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errJWTInvalidClaims
	}

	return claims, nil
}

func (j *JWTManager) GetJTI(tokenString string) (string, error) {
	claims, err := j.Validate(tokenString)
	if err != nil {
		return "", err
	}
	return claims.ID, nil
}
