package stt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	errGoogleAuthTokenEndpoint = errors.New("token endpoint error")
	errGoogleAuthEmptyToken    = errors.New("empty access_token in response")
	errGoogleAuthDecodePEM     = errors.New("failed to decode PEM block from private key")
	errGoogleAuthNotRSA        = errors.New("private key is not RSA")
)

// signServiceAccountJWT signs a JWT with the service account private key and
// exchanges it for an OAuth2 access token via the Google token endpoint.
// Pure-Go implementation — no golang.org/x/oauth2 dependency.
func signServiceAccountJWT(ctx context.Context, client *http.Client, sa *googleServiceAccount, scope string) (string, error) {
	now := time.Now().Unix()
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token" //nolint:gosec // G101 false positive: public OAuth endpoint URL, not a credential
	}

	header := base64url(mustJSON(map[string]string{"alg": "RS256", "typ": "JWT"}))
	claims := base64url(mustJSON(map[string]interface{}{
		"iss": sa.ClientEmail,
		"scope": scope,
		"aud": tokenURI,
		"iat": now,
		"exp": now + 3600,
	}))

	signingInput := header + "." + claims

	privateKey, err := parseRSAPrivateKey(sa.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	signedJWT := signingInput + "." + base64url(sig)

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signedJWT},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("new token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("token response read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d: %s", errGoogleAuthTokenEndpoint, resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("%w: %s", errGoogleAuthEmptyToken, string(body))
	}
	return tokenResp.AccessToken, nil
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func parseRSAPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		// Some service account keys store literal \n instead of real newlines.
		block, _ = pem.Decode([]byte(strings.ReplaceAll(pemKey, `\n`, "\n")))
	}
	if block == nil {
		return nil, errGoogleAuthDecodePEM
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w (got %T)", errGoogleAuthNotRSA, key)
	}
	return rsaKey, nil
}
