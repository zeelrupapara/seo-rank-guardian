package oauth2

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenClaims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type OAuth2 struct {
	accessSecret  string
	refreshSecret string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	redis         *redis.Client
	log           *zap.SugaredLogger
}

func NewOAuth2(cfg config.OAuthConfig, redisClient *redis.Client, log *zap.SugaredLogger) (*OAuth2, error) {
	accessExpiry, err := time.ParseDuration(cfg.AccessExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid access expiry: %w", err)
	}

	refreshExpiry, err := time.ParseDuration(cfg.RefreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh expiry: %w", err)
	}

	return &OAuth2{
		accessSecret:  cfg.AccessSecret,
		refreshSecret: cfg.RefreshSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		redis:         redisClient,
		log:           log,
	}, nil
}

func (o *OAuth2) GenerateTokenPair(userID uint, role string) (*TokenPair, error) {
	accessToken, err := o.generateToken(userID, role, o.accessSecret, o.accessExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := o.generateToken(userID, role, o.refreshSecret, o.refreshExpiry)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := fmt.Sprintf("session:%d", userID)
	if err := o.redis.Set(ctx, key, refreshToken, o.refreshExpiry).Err(); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (o *OAuth2) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
	return o.validateToken(tokenStr, o.accessSecret)
}

func (o *OAuth2) ValidateRefreshToken(tokenStr string) (*TokenClaims, error) {
	claims, err := o.validateToken(tokenStr, o.refreshSecret)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := fmt.Sprintf("session:%d", claims.UserID)
	stored, err := o.redis.Get(ctx, key).Result()
	if err != nil || stored != tokenStr {
		return nil, fmt.Errorf("invalid refresh token")
	}

	return claims, nil
}

func (o *OAuth2) RevokeSession(userID uint) error {
	ctx := context.Background()
	key := fmt.Sprintf("session:%d", userID)
	return o.redis.Del(ctx, key).Err()
}

func (o *OAuth2) generateToken(userID uint, role, secret string, expiry time.Duration) (string, error) {
	claims := TokenClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func (o *OAuth2) validateToken(tokenStr, secret string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
