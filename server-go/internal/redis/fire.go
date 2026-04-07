// Package redis provides a concrete Redis client implementation for the hwarr server.
//
// fire.go implements the fire:{gridId} Sorted Set operations.
// Each fire grid is stored as a Redis Sorted Set where:
//   - Key:    fire:{gridId}   (e.g., "fire:37.576:126.977")
//   - Member: unique event ID (e.g., "fire-a1b2c3d4e5f6")
//   - Score:  expiration timestamp (Unix seconds, typically now + 2400)
//
// Active fires are counted via ZCOUNT with score range [now, +inf].
// Expired fires are cleaned up via ZREMRANGEBYSCORE with range [-inf, now].
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ZMember represents a sorted set member with its score.
// Used by ZRevRangeWithScores and compatible with handler.ZMember.
type ZMember struct {
	Member string
	Score  float64
}

// Client wraps a go-redis client and provides methods that satisfy
// all fire-related Redis interfaces defined across the codebase.
type Client struct {
	rdb *goredis.Client
}

// NewClient creates a new Redis client connected to the given address.
// addr is in "host:port" format (e.g., "localhost:6379").
func NewClient(addr, password string, db int) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
	})
	return &Client{rdb: rdb}
}

// NewClientFromRedis creates a Client from an existing go-redis client.
func NewClientFromRedis(rdb *goredis.Client) *Client {
	return &Client{rdb: rdb}
}

// Close closes the underlying Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks the Redis connection.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// --- Sorted Set operations for fire:{gridId} ---

// ZAdd adds a member with score to a sorted set.
// Used to register a new fire event: ZADD fire:{gridId} <expireAt> <eventId>
func (c *Client) ZAdd(ctx context.Context, key string, score float64, member string) error {
	return c.rdb.ZAdd(ctx, key, goredis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// ZCount returns the number of elements in the sorted set at key
// with a score between min and max (inclusive, string-encoded).
// Used to count active fires: ZCOUNT fire:{gridId} <now> +inf
func (c *Client) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return c.rdb.ZCount(ctx, key, min, max).Result()
}

// ZCard returns the total number of members in a sorted set.
// Used after cleanup to check if a fire grid is empty.
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	return c.rdb.ZCard(ctx, key).Result()
}

// ZRemRangeByScore removes all members in a sorted set with score between min and max.
// Returns the number of removed members.
// Used to clean up expired fires: ZREMRANGEBYSCORE fire:{gridId} -inf <now>
func (c *Client) ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error) {
	return c.rdb.ZRemRangeByScore(ctx, key, min, max).Result()
}

// ZRevRangeWithScores returns the specified range of elements in the sorted set
// stored at key, ordered from highest to lowest score.
// Used to get the latest fire event timestamp.
func (c *Client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZMember, error) {
	result, err := c.rdb.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}
	members := make([]ZMember, len(result))
	for i, z := range result {
		members[i] = ZMember{
			Member: fmt.Sprintf("%v", z.Member),
			Score:  z.Score,
		}
	}
	return members, nil
}

// ZIncrBy increments the score of a member in a sorted set by increment.
// Used for daily ranking: ZINCRBY stats:daily_ranking:<date> 1 <region>
func (c *Client) ZIncrBy(ctx context.Context, key string, increment float64, member string) error {
	return c.rdb.ZIncrBy(ctx, key, increment, member).Err()
}

// --- Set operations (for active_grids tracking) ---

// SAdd adds one or more members to a set.
// Used to track active grids: SADD active_grids <gridId>
func (c *Client) SAdd(ctx context.Context, key string, member string) error {
	return c.rdb.SAdd(ctx, key, member).Err()
}

// SMembers returns all members of the set at key.
// Used to list all active grid IDs.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, key).Result()
}

// SRem removes one or more members from a set.
// Used to remove empty grids from active tracking.
// Accepts variadic interface{} to match engine.RedisProgressionReader.
func (c *Client) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return c.rdb.SRem(ctx, key, members...).Result()
}

// --- String/counter operations ---

// Get returns the string value of key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result, err := c.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", nil
	}
	return result, err
}

// Incr increments the integer value of key by one.
// Used for stats counters: INCR stats:total_fires
func (c *Client) Incr(ctx context.Context, key string) error {
	return c.rdb.Incr(ctx, key).Err()
}

// Set sets key to hold the string value with optional expiration.
func (c *Client) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Expire sets a timeout on key.
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.rdb.Expire(ctx, key, expiration).Err()
}

// --- List operations (for chat messages) ---

// LPush inserts values at the head of a list.
func (c *Client) LPush(ctx context.Context, key string, values ...interface{}) error {
	return c.rdb.LPush(ctx, key, values...).Err()
}

// LTrim trims a list to the specified range.
func (c *Client) LTrim(ctx context.Context, key string, start, stop int64) error {
	return c.rdb.LTrim(ctx, key, start, stop).Err()
}

// LRange returns the specified range of elements in a list.
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.LRange(ctx, key, start, stop).Result()
}

// IncrBy increments the integer value of a key by the given amount.
func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, value).Result()
}

// SCard returns the number of members in a set.
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	return c.rdb.SCard(ctx, key).Result()
}

// Underlying returns the raw go-redis client for advanced operations.
func (c *Client) Underlying() *goredis.Client {
	return c.rdb
}
