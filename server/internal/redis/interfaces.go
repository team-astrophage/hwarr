package redis

// This file provides thin adapter wrappers for Redis interfaces that have
// incompatible method signatures (e.g., different SRem arities).
//
// With model.ZMember as the shared type, Client directly satisfies most
// consumer interfaces (handler.RedisGridReader, handler.RedisStatsReader,
// handler.RedisNewsReader, handler.RedisRankingReader, sio.RedisFireStateReader, etc.).
// Only CleanupAdapter and ProgressionAdapter remain, bridging SRem signature differences.

import (
	"context"
	"time"

	"github.com/homepy/hwarr/server/internal/model"
)

// --- Adapter for engine.CleanupRedis (SRem has single-member signature) ---

// CleanupAdapter wraps Client to satisfy engine.CleanupRedis,
// which uses SRem(ctx, key, member string) error instead of variadic.
type CleanupAdapter struct {
	*Client
}

// SRem adapts the Client's variadic SRem to single-member signature.
func (a *CleanupAdapter) SRem(ctx context.Context, key, member string) error {
	_, err := a.Client.SRem(ctx, key, member)
	return err
}

// AsCleanupRedis returns an adapter that satisfies engine.CleanupRedis.
func (c *Client) AsCleanupRedis() *CleanupAdapter {
	return &CleanupAdapter{Client: c}
}

// --- Adapter for engine.RedisProgressionReader ---

// ProgressionAdapter wraps Client to satisfy engine.RedisProgressionReader.
type ProgressionAdapter struct {
	c *Client
}

func (a *ProgressionAdapter) SMembers(ctx context.Context, key string) ([]string, error) {
	return a.c.SMembers(ctx, key)
}

func (a *ProgressionAdapter) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return a.c.ZCount(ctx, key, min, max)
}

func (a *ProgressionAdapter) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return a.c.SRem(ctx, key, members...)
}

// AsProgressionReader returns an adapter that satisfies engine.RedisProgressionReader.
func (c *Client) AsProgressionReader() *ProgressionAdapter {
	return &ProgressionAdapter{c: c}
}

// --- Adapter for feedback rate limiter (Expire takes int seconds) ---

// FeedbackRateLimitAdapter bridges Client to handler.RedisFeedbackRateLimiter.
type FeedbackRateLimitAdapter struct {
	c *Client
}

func (a *FeedbackRateLimitAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.c.IncrBy(ctx, key, 1)
}

func (a *FeedbackRateLimitAdapter) Expire(ctx context.Context, key string, seconds int) error {
	return a.c.Expire(ctx, key, secondsToDuration(seconds))
}

// AsFeedbackRateLimiter returns an adapter for handler.RedisFeedbackRateLimiter.
func (c *Client) AsFeedbackRateLimiter() *FeedbackRateLimitAdapter {
	return &FeedbackRateLimitAdapter{c: c}
}

// --- Adapter for chat (Expire takes int seconds) ---

// ChatAdapter bridges Client to sio.RedisChatWriter.
type ChatAdapter struct {
	c *Client
}

func (a *ChatAdapter) LPush(ctx context.Context, key string, value string) error {
	return a.c.LPush(ctx, key, value)
}

func (a *ChatAdapter) LTrim(ctx context.Context, key string, start, stop int64) error {
	return a.c.LTrim(ctx, key, start, stop)
}

func (a *ChatAdapter) Expire(ctx context.Context, key string, seconds int) error {
	return a.c.Expire(ctx, key, secondsToDuration(seconds))
}

func (a *ChatAdapter) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.c.LRange(ctx, key, start, stop)
}

// AsChatWriter returns an adapter for sio.RedisChatWriter.
func (c *Client) AsChatWriter() *ChatAdapter {
	return &ChatAdapter{c: c}
}

// --- Adapter for news reader (ZRevRangeWithScores returns model.ZMember) ---

// NewsReaderAdapter wraps Client to satisfy handler.RedisNewsReader.
type NewsReaderAdapter struct {
	c *Client
}

func (a *NewsReaderAdapter) SMembers(ctx context.Context, key string) ([]string, error) {
	return a.c.SMembers(ctx, key)
}

func (a *NewsReaderAdapter) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return a.c.ZCount(ctx, key, min, max)
}

func (a *NewsReaderAdapter) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]model.ZMember, error) {
	return a.c.ZRevRangeWithScores(ctx, key, start, stop)
}

// AsNewsReader returns an adapter that satisfies handler.RedisNewsReader.
func (c *Client) AsNewsReader() *NewsReaderAdapter {
	return &NewsReaderAdapter{c: c}
}

// --- Adapter for ranking reader ---

// RankingReaderAdapter wraps Client to satisfy handler.RedisRankingReader.
type RankingReaderAdapter struct {
	c *Client
}

func (a *RankingReaderAdapter) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]model.ZMember, error) {
	return a.c.ZRevRangeWithScores(ctx, key, start, stop)
}

// AsRankingReader returns an adapter that satisfies handler.RedisRankingReader.
func (c *Client) AsRankingReader() *RankingReaderAdapter {
	return &RankingReaderAdapter{c: c}
}

func secondsToDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
