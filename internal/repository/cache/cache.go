// Package cache provides Redis-based caching for WeChat tokens.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key format constants
const (
	ComponentTokenKeyFormat  = "wechat:token:component:%s"  // wechat:token:component:{component_appid}
	AuthorizerTokenKeyFormat = "wechat:token:authorizer:%s" // wechat:token:authorizer:{authorizer_appid}
)

// SafetyMargin is the time to subtract from token TTL for safety
const SafetyMargin = 5 * time.Minute

// Repository defines the cache repository interface.
type Repository interface {
	// GetComponentToken retrieves cached component_access_token
	GetComponentToken(ctx context.Context, componentAppID string) (string, error)

	// SetComponentToken caches component_access_token with TTL
	SetComponentToken(ctx context.Context, componentAppID string, token string, expiresIn int) error

	// GetAuthorizerToken retrieves cached authorizer_access_token
	GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error)

	// SetAuthorizerToken caches authorizer_access_token with TTL
	SetAuthorizerToken(ctx context.Context, authorizerAppID string, token string, expiresIn int) error

	// GetTokenTTL returns the remaining TTL for a token
	GetTokenTTL(ctx context.Context, key string) (time.Duration, error)

	// DeleteToken deletes a cached token
	DeleteToken(ctx context.Context, key string) error

	// Close closes the Redis connection
	Close() error
}

// RedisRepository implements Repository using Redis.
type RedisRepository struct {
	client *redis.Client
}

// NewRedisRepository creates a new Redis repository.
func NewRedisRepository(addr, password string, db int) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisRepository{client: client}, nil
}

// GetComponentToken retrieves cached component_access_token.
func (r *RedisRepository) GetComponentToken(ctx context.Context, componentAppID string) (string, error) {
	key := FormatComponentTokenKey(componentAppID)
	token, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Not found, return empty string
	}
	if err != nil {
		return "", fmt.Errorf("failed to get component token: %w", err)
	}
	return token, nil
}

// SetComponentToken caches component_access_token with TTL.
func (r *RedisRepository) SetComponentToken(ctx context.Context, componentAppID string, token string, expiresIn int) error {
	key := FormatComponentTokenKey(componentAppID)
	ttl := CalculateTTL(expiresIn)

	if err := r.client.Set(ctx, key, token, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set component token: %w", err)
	}
	return nil
}

// GetAuthorizerToken retrieves cached authorizer_access_token.
func (r *RedisRepository) GetAuthorizerToken(ctx context.Context, authorizerAppID string) (string, error) {
	key := FormatAuthorizerTokenKey(authorizerAppID)
	token, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Not found, return empty string
	}
	if err != nil {
		return "", fmt.Errorf("failed to get authorizer token: %w", err)
	}
	return token, nil
}

// SetAuthorizerToken caches authorizer_access_token with TTL.
func (r *RedisRepository) SetAuthorizerToken(ctx context.Context, authorizerAppID string, token string, expiresIn int) error {
	key := FormatAuthorizerTokenKey(authorizerAppID)
	ttl := CalculateTTL(expiresIn)

	if err := r.client.Set(ctx, key, token, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set authorizer token: %w", err)
	}
	return nil
}

// GetTokenTTL returns the remaining TTL for a token.
func (r *RedisRepository) GetTokenTTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}
	return ttl, nil
}

// DeleteToken deletes a cached token.
func (r *RedisRepository) DeleteToken(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}

// Close closes the Redis connection.
func (r *RedisRepository) Close() error {
	return r.client.Close()
}

// FormatComponentTokenKey generates the Redis key for component token.
func FormatComponentTokenKey(componentAppID string) string {
	return fmt.Sprintf(ComponentTokenKeyFormat, componentAppID)
}

// FormatAuthorizerTokenKey generates the Redis key for authorizer token.
func FormatAuthorizerTokenKey(authorizerAppID string) string {
	return fmt.Sprintf(AuthorizerTokenKeyFormat, authorizerAppID)
}

// CalculateTTL calculates the cache TTL from expires_in with safety margin.
func CalculateTTL(expiresIn int) time.Duration {
	ttl := time.Duration(expiresIn)*time.Second - SafetyMargin
	if ttl < 0 {
		ttl = time.Duration(expiresIn) * time.Second / 2 // Fallback to half of expires_in
	}
	return ttl
}
