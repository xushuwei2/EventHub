package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/eventhub/eventhub/internal/models"
)

var (
	ErrInvalidToken = errors.New("invalid_token")
	ErrInvalidCreds = errors.New("invalid_credentials")
)

type AdminSession struct {
	Username  string
	ExpiresAt time.Time
}

func VerifyAdminPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashAdminPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func SignAdminSession(secret, username string, ttl time.Duration) (string, error) {
	payload := fmt.Sprintf("%s|%d", username, time.Now().Add(ttl).Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig)), nil
}

func ParseAdminSession(secret, token string) (*AdminSession, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidCreds
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return nil, ErrInvalidCreds
	}
	expUnix, err := parseInt64(parts[1])
	if err != nil {
		return nil, ErrInvalidCreds
	}
	payload := strings.Join(parts[:2], "|")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, ErrInvalidCreds
	}
	exp := time.Unix(expUnix, 0)
	if time.Now().After(exp) {
		return nil, ErrInvalidCreds
	}
	return &AdminSession{Username: parts[0], ExpiresAt: exp}, nil
}

func SignReportToken(secret string, id *models.TrustedIdentity) (string, error) {
	if id == nil || id.ProjectKey == "" {
		return "", ErrInvalidToken
	}
	claims := jwt.MapClaims{
		"project_key": id.ProjectKey,
	}
	if id.UserID != "" {
		claims["user_id"] = id.UserID
	}
	if id.SessionID != "" {
		claims["session_id"] = id.SessionID
	}
	if id.RoomID != "" {
		claims["room_id"] = id.RoomID
	}
	if id.Release != "" {
		claims["release"] = id.Release
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseReportToken(secret, token string) (*models.TrustedIdentity, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	parsed, err := parser.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	projectKey, _ := claims["project_key"].(string)
	if projectKey == "" {
		return nil, ErrInvalidToken
	}

	id := &models.TrustedIdentity{ProjectKey: projectKey}
	if v, ok := claims["user_id"].(string); ok {
		id.UserID = v
	}
	if v, ok := claims["session_id"].(string); ok {
		id.SessionID = v
	}
	if v, ok := claims["room_id"].(string); ok {
		id.RoomID = v
	}
	if v, ok := claims["release"].(string); ok {
		id.Release = v
	}
	return id, nil
}

func NewRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
