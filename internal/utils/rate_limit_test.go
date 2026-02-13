package utils

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func withTestViper(t *testing.T) {
	t.Helper()
	old := V
	V = viper.New()
	t.Cleanup(func() {
		V = old
	})
}

func TestLoadRateLimitConfigFromViper(t *testing.T) {
	withTestViper(t)
	V.Set(cfgRateLimitRequestPerMin, 120)
	V.Set(cfgRateLimitTokenPerMin, 450)
	V.Set(cfgRateLimitTokenK, 50)
	V.Set(cfgRateLimitDefaultMaxToken, 512)
	V.Set(cfgRateLimitWindowSeconds, 30)
	V.Set(cfgRateLimitRedisPrefix, "rl:test")

	cfg := loadRateLimitConfigFromViper()
	if cfg.RequestPerMin != 120 {
		t.Fatalf("expected request_per_min=120, got %d", cfg.RequestPerMin)
	}
	if cfg.TokenPerMin != 450 {
		t.Fatalf("expected token_per_min=450, got %d", cfg.TokenPerMin)
	}
	if cfg.TokenK != 50 {
		t.Fatalf("expected token_k=50, got %d", cfg.TokenK)
	}
	if cfg.DefaultMaxTokens != 512 {
		t.Fatalf("expected default_max_tokens=512, got %d", cfg.DefaultMaxTokens)
	}
	if cfg.WindowSeconds != 30 {
		t.Fatalf("expected window_seconds=30, got %d", cfg.WindowSeconds)
	}
	if cfg.RedisPrefix != "rl:test" {
		t.Fatalf("expected redis_prefix=rl:test, got %q", cfg.RedisPrefix)
	}
}

func TestLoadRateLimitConfigFromViperFallbacks(t *testing.T) {
	withTestViper(t)
	V.Set(cfgRateLimitRequestPerMin, -1)
	V.Set(cfgRateLimitTokenPerMin, -99)
	V.Set(cfgRateLimitTokenK, 0)
	V.Set(cfgRateLimitDefaultMaxToken, -1)
	V.Set(cfgRateLimitWindowSeconds, 0)
	V.Set(cfgRateLimitRedisPrefix, " ")

	cfg := loadRateLimitConfigFromViper()
	if cfg.RequestPerMin != defaultRateLimitRequestPerMin {
		t.Fatalf("expected request_per_min default %d, got %d", defaultRateLimitRequestPerMin, cfg.RequestPerMin)
	}
	if cfg.TokenPerMin != defaultRateLimitTokenPerMin {
		t.Fatalf("expected token_per_min default %d, got %d", defaultRateLimitTokenPerMin, cfg.TokenPerMin)
	}
	if cfg.TokenK != defaultRateLimitTokenK {
		t.Fatalf("expected token_k default %d, got %d", defaultRateLimitTokenK, cfg.TokenK)
	}
	if cfg.DefaultMaxTokens != defaultRateLimitDefaultMaxToken {
		t.Fatalf("expected default_max_tokens default %d, got %d", defaultRateLimitDefaultMaxToken, cfg.DefaultMaxTokens)
	}
	if cfg.WindowSeconds != defaultRateLimitWindowSeconds {
		t.Fatalf("expected window_seconds default %d, got %d", defaultRateLimitWindowSeconds, cfg.WindowSeconds)
	}
	if cfg.RedisPrefix != defaultRateLimitRedisPrefix {
		t.Fatalf("expected redis_prefix default %q, got %q", defaultRateLimitRedisPrefix, cfg.RedisPrefix)
	}
}

func TestCalculateTokenCost(t *testing.T) {
	tests := []struct {
		name      string
		promptEst int64
		maxTokens int64
		k         int64
		want      int64
	}{
		{
			name:      "zero total",
			promptEst: 0,
			maxTokens: 0,
			k:         100,
			want:      0,
		},
		{
			name:      "one token",
			promptEst: 1,
			maxTokens: 0,
			k:         100,
			want:      1,
		},
		{
			name:      "exactly one chunk",
			promptEst: 99,
			maxTokens: 1,
			k:         100,
			want:      1,
		},
		{
			name:      "need ceil",
			promptEst: 100,
			maxTokens: 1,
			k:         100,
			want:      2,
		},
		{
			name:      "custom k",
			promptEst: 120,
			maxTokens: 80,
			k:         50,
			want:      4,
		},
	}

	for _, tt := range tests {
		got := CalculateTokenCost(tt.promptEst, tt.maxTokens, tt.k)
		if got != tt.want {
			t.Fatalf("%s: expected %d, got %d", tt.name, tt.want, got)
		}
	}
}

func TestBuildRateLimitWindowKeys(t *testing.T) {
	cfg := RateLimitConfig{
		WindowSeconds: 60,
		RedisPrefix:   "rl:chat",
	}

	req1, tok1, win1 := BuildRateLimitWindowKeys(cfg, 42, time.Unix(119, 0).UTC())
	req2, tok2, win2 := BuildRateLimitWindowKeys(cfg, 42, time.Unix(120, 0).UTC())

	if win1 != 1 || win2 != 2 {
		t.Fatalf("unexpected window ids: win1=%d win2=%d", win1, win2)
	}
	if req1 == req2 {
		t.Fatalf("expected different request keys across windows, got %q and %q", req1, req2)
	}
	if tok1 == tok2 {
		t.Fatalf("expected different token keys across windows, got %q and %q", tok1, tok2)
	}
	if req1 != "rl:chat:req:42:1" {
		t.Fatalf("unexpected request key: %q", req1)
	}
	if tok1 != "rl:chat:tok:42:1" {
		t.Fatalf("unexpected token key: %q", tok1)
	}
}
