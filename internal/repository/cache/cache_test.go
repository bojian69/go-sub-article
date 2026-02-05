package cache

import (
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
)

// Property 2: Cache Key Format Includes Identifier
// For any authorizer_appid, the generated cache key SHALL contain that authorizer_appid.
// For any component_appid, the generated cache key SHALL contain that component_appid.
// **Validates: Requirements 1.6, 7.3, 7.4**
func TestProperty_CacheKeyFormatIncludesIdentifier(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: Component token key contains component_appid
	properties.Property("component token key contains appid", prop.ForAll(
		func(appID string) bool {
			if appID == "" {
				return true // Skip empty strings
			}
			key := FormatComponentTokenKey(appID)
			return strings.Contains(key, appID) &&
				strings.HasPrefix(key, "wechat:token:component:")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
	))

	// Property: Authorizer token key contains authorizer_appid
	properties.Property("authorizer token key contains appid", prop.ForAll(
		func(appID string) bool {
			if appID == "" {
				return true // Skip empty strings
			}
			key := FormatAuthorizerTokenKey(appID)
			return strings.Contains(key, appID) &&
				strings.HasPrefix(key, "wechat:token:authorizer:")
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
	))

	// Property: Different appids produce different keys
	// Use a counter to ensure unique appids
	var counter int
	properties.Property("different appids produce different keys", prop.ForAll(
		func(base string) bool {
			if base == "" {
				return true // Skip empty strings
			}
			counter++
			appID1 := base + "_1"
			appID2 := base + "_2"
			key1 := FormatAuthorizerTokenKey(appID1)
			key2 := FormatAuthorizerTokenKey(appID2)
			return key1 != key2
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// Property 15: TTL Calculation
// For any token cached in Redis, the TTL SHALL be set to (expires_in - 5 minutes).
// **Validates: Requirements 7.2**
func TestProperty_TTLCalculation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(12345)

	properties := gopter.NewProperties(parameters)

	// Property: TTL = expires_in - 5 minutes for normal cases
	properties.Property("TTL equals expires_in minus 5 minutes", prop.ForAll(
		func(expiresIn int) bool {
			if expiresIn <= 0 {
				return true // Skip invalid values
			}

			ttl := CalculateTTL(expiresIn)
			expected := time.Duration(expiresIn)*time.Second - SafetyMargin

			// For values where expected would be negative, use fallback
			if expected < 0 {
				expected = time.Duration(expiresIn) * time.Second / 2
			}

			return ttl == expected
		},
		gen.IntRange(1, 7200), // 1 second to 2 hours
	))

	// Property: TTL is always positive for reasonable expires_in
	properties.Property("TTL is always positive for expires_in > 10 minutes", prop.ForAll(
		func(expiresIn int) bool {
			ttl := CalculateTTL(expiresIn)
			return ttl > 0
		},
		gen.IntRange(601, 7200), // 10+ minutes to 2 hours
	))

	// Property: TTL is less than expires_in
	properties.Property("TTL is less than expires_in", prop.ForAll(
		func(expiresIn int) bool {
			if expiresIn <= 0 {
				return true
			}
			ttl := CalculateTTL(expiresIn)
			return ttl < time.Duration(expiresIn)*time.Second
		},
		gen.IntRange(1, 7200),
	))

	properties.TestingRun(t)
}

// Unit tests for specific edge cases
func TestFormatComponentTokenKey(t *testing.T) {
	tests := []struct {
		appID    string
		expected string
	}{
		{"wx123456", "wechat:token:component:wx123456"},
		{"test_app", "wechat:token:component:test_app"},
	}

	for _, tt := range tests {
		t.Run(tt.appID, func(t *testing.T) {
			result := FormatComponentTokenKey(tt.appID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAuthorizerTokenKey(t *testing.T) {
	tests := []struct {
		appID    string
		expected string
	}{
		{"wx789012", "wechat:token:authorizer:wx789012"},
		{"auth_app", "wechat:token:authorizer:auth_app"},
	}

	for _, tt := range tests {
		t.Run(tt.appID, func(t *testing.T) {
			result := FormatAuthorizerTokenKey(tt.appID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateTTL(t *testing.T) {
	tests := []struct {
		name      string
		expiresIn int
		expected  time.Duration
	}{
		{
			name:      "2 hours (7200 seconds)",
			expiresIn: 7200,
			expected:  7200*time.Second - SafetyMargin,
		},
		{
			name:      "1 hour (3600 seconds)",
			expiresIn: 3600,
			expected:  3600*time.Second - SafetyMargin,
		},
		{
			name:      "10 minutes (600 seconds)",
			expiresIn: 600,
			expected:  600*time.Second - SafetyMargin,
		},
		{
			name:      "less than safety margin - fallback",
			expiresIn: 200, // 200 seconds < 5 minutes
			expected:  100 * time.Second, // Half of 200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateTTL(tt.expiresIn)
			assert.Equal(t, tt.expected, result)
		})
	}
}
