package redis

// This file provides compile-time interface satisfaction checks and adapter
// types for Redis interfaces that have incompatible method signatures.
//
// The core Client satisfies most interfaces directly. For interfaces with
// different SRem signatures, we provide thin adapter wrappers.

import (
	"context"

	"github.com/homepy/hwarr/server-go/internal/engine"
	"github.com/homepy/hwarr/server-go/internal/handler"
	"github.com/homepy/hwarr/server-go/internal/sio"
)

// --- Direct interface satisfaction checks ---

// handler.RedisGridReader: ZCount
var _ handler.RedisGridReader = (*Client)(nil)

// handler.RedisStatsReader: SMembers, ZCount, Get
var _ handler.RedisStatsReader = (*Client)(nil)

// sio.RedisFireStateReader: SMembers, ZCount
var _ sio.RedisFireStateReader = (*Client)(nil)

// --- Adapter for engine.CleanupRedis (SRem has different signature) ---

// CleanupAdapter wraps Client to satisfy engine.CleanupRedis,
// which uses SRem(ctx, key, member string) error instead of variadic.
type CleanupAdapter struct {
	*Client
}

// SRem adapts the Client's variadic SRem to CleanupRedis's single-member signature.
func (a *CleanupAdapter) SRem(ctx context.Context, key, member string) error {
	_, err := a.Client.SRem(ctx, key, member)
	return err
}

var _ engine.CleanupRedis = (*CleanupAdapter)(nil)

// AsCleanupRedis returns an adapter that satisfies engine.CleanupRedis.
func (c *Client) AsCleanupRedis() *CleanupAdapter {
	return &CleanupAdapter{Client: c}
}

// --- Adapter for engine.RedisProgressionReader ---

// ProgressionAdapter wraps Client to satisfy engine.RedisProgressionReader,
// which uses SRem(ctx, key string, members ...interface{}) (int64, error).
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

var _ engine.RedisProgressionReader = (*ProgressionAdapter)(nil)

// AsProgressionReader returns an adapter that satisfies engine.RedisProgressionReader.
func (c *Client) AsProgressionReader() *ProgressionAdapter {
	return &ProgressionAdapter{c: c}
}

// --- Adapter for handler.RedisNewsReader (ZRevRangeWithScores returns handler.ZMember) ---

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

func (a *NewsReaderAdapter) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]handler.ZMember, error) {
	members, err := a.c.ZRevRangeWithScores(ctx, key, start, stop)
	if err != nil {
		return nil, err
	}
	result := make([]handler.ZMember, len(members))
	for i, m := range members {
		result[i] = handler.ZMember{Member: m.Member, Score: m.Score}
	}
	return result, nil
}

var _ handler.RedisNewsReader = (*NewsReaderAdapter)(nil)

// AsNewsReader returns an adapter that satisfies handler.RedisNewsReader.
func (c *Client) AsNewsReader() *NewsReaderAdapter {
	return &NewsReaderAdapter{c: c}
}

// --- Adapter for handler.RedisRankingReader ---

// RankingReaderAdapter wraps Client to satisfy handler.RedisRankingReader.
type RankingReaderAdapter struct {
	c *Client
}

func (a *RankingReaderAdapter) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]handler.ZMember, error) {
	members, err := a.c.ZRevRangeWithScores(ctx, key, start, stop)
	if err != nil {
		return nil, err
	}
	result := make([]handler.ZMember, len(members))
	for i, m := range members {
		result[i] = handler.ZMember{Member: m.Member, Score: m.Score}
	}
	return result, nil
}

var _ handler.RedisRankingReader = (*RankingReaderAdapter)(nil)

// AsRankingReader returns an adapter that satisfies handler.RedisRankingReader.
func (c *Client) AsRankingReader() *RankingReaderAdapter {
	return &RankingReaderAdapter{c: c}
}
