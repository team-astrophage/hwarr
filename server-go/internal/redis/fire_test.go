package redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// testRedisAddr returns the Redis address for integration tests.
// Defaults to localhost:6379, overridable via REDIS_URL env var.
func testRedisAddr() string {
	if addr := os.Getenv("REDIS_URL"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

// skipIfNoRedis skips the test if Redis is not available.
func skipIfNoRedis(t *testing.T) *goredis.Client {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{
		Addr:        testRedisAddr(),
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s: %v", testRedisAddr(), err)
	}
	return rdb
}

// testKey generates a unique test key to avoid collisions.
func testKey(prefix string) string {
	return fmt.Sprintf("test:%s:%d", prefix, time.Now().UnixNano())
}

func TestZAdd_And_ZCount(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	key := testKey("fire:9:9")
	defer rdb.Del(ctx, key)

	now := float64(time.Now().Unix())
	expireAt := now + 2400 // 40 minutes from now

	// Add 3 fire events with future expiration (active)
	for i := 0; i < 3; i++ {
		eventID := fmt.Sprintf("fire-test-%d", i)
		err := client.ZAdd(ctx, key, expireAt+float64(i), eventID)
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}
	}

	// Add 2 expired fire events (score < now)
	for i := 0; i < 2; i++ {
		eventID := fmt.Sprintf("fire-expired-%d", i)
		err := client.ZAdd(ctx, key, now-100-float64(i), eventID)
		if err != nil {
			t.Fatalf("ZAdd expired failed: %v", err)
		}
	}

	// ZCount should return only active fires (score >= now)
	nowStr := fmt.Sprintf("%f", now)
	activeCount, err := client.ZCount(ctx, key, nowStr, "+inf")
	if err != nil {
		t.Fatalf("ZCount failed: %v", err)
	}
	if activeCount != 3 {
		t.Errorf("expected 3 active fires, got %d", activeCount)
	}

	// ZCard should return total (active + expired)
	total, err := client.ZCard(ctx, key)
	if err != nil {
		t.Fatalf("ZCard failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 total fires, got %d", total)
	}
}

func TestZRemRangeByScore_Cleanup(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	key := testKey("fire:10:10")
	defer rdb.Del(ctx, key)

	now := float64(time.Now().Unix())

	// Add active fires (future score)
	for i := 0; i < 3; i++ {
		err := client.ZAdd(ctx, key, now+2400+float64(i), fmt.Sprintf("active-%d", i))
		if err != nil {
			t.Fatalf("ZAdd active failed: %v", err)
		}
	}

	// Add expired fires (past score)
	for i := 0; i < 5; i++ {
		err := client.ZAdd(ctx, key, now-float64(i+1), fmt.Sprintf("expired-%d", i))
		if err != nil {
			t.Fatalf("ZAdd expired failed: %v", err)
		}
	}

	// Remove expired: ZREMRANGEBYSCORE key -inf <now>
	nowStr := fmt.Sprintf("%f", now)
	removed, err := client.ZRemRangeByScore(ctx, key, "-inf", nowStr)
	if err != nil {
		t.Fatalf("ZRemRangeByScore failed: %v", err)
	}
	if removed != 5 {
		t.Errorf("expected 5 removed, got %d", removed)
	}

	// Verify only active fires remain
	remaining, err := client.ZCard(ctx, key)
	if err != nil {
		t.Fatalf("ZCard after cleanup failed: %v", err)
	}
	if remaining != 3 {
		t.Errorf("expected 3 remaining, got %d", remaining)
	}
}

func TestZRevRangeWithScores(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	key := testKey("fire:11:11")
	defer rdb.Del(ctx, key)

	now := float64(time.Now().Unix())

	// Add fires with different scores
	scores := []float64{now + 100, now + 200, now + 300}
	for i, score := range scores {
		err := client.ZAdd(ctx, key, score, fmt.Sprintf("fire-%d", i))
		if err != nil {
			t.Fatalf("ZAdd failed: %v", err)
		}
	}

	// Get the highest-scored member (latest fire)
	members, err := client.ZRevRangeWithScores(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ZRevRangeWithScores failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Member != "fire-2" {
		t.Errorf("expected fire-2, got %s", members[0].Member)
	}
	if members[0].Score != scores[2] {
		t.Errorf("expected score %f, got %f", scores[2], members[0].Score)
	}
}

func TestEmptyGrid_Cleanup(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	fireKey := testKey("fire:12:12")
	activeGridsKey := testKey("active_grids")
	defer rdb.Del(ctx, fireKey, activeGridsKey)

	now := float64(time.Now().Unix())

	// Setup: add one expired fire and track the grid
	err := client.ZAdd(ctx, fireKey, now-100, "expired-fire-1")
	if err != nil {
		t.Fatalf("ZAdd failed: %v", err)
	}
	err = client.SAdd(ctx, activeGridsKey, "12:12")
	if err != nil {
		t.Fatalf("SAdd failed: %v", err)
	}

	// Cleanup expired fires
	nowStr := fmt.Sprintf("%f", now)
	removed, err := client.ZRemRangeByScore(ctx, fireKey, "-inf", nowStr)
	if err != nil {
		t.Fatalf("ZRemRangeByScore failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Verify grid is empty
	remaining, err := client.ZCard(ctx, fireKey)
	if err != nil {
		t.Fatalf("ZCard failed: %v", err)
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}

	// Clean up empty grid: delete key and remove from active set
	err = client.Del(ctx, fireKey)
	if err != nil {
		t.Fatalf("Del failed: %v", err)
	}

	// Remove from active_grids using SRem with interface{} arg (matches engine interface)
	n, err := client.SRem(ctx, activeGridsKey, "12:12")
	if err != nil {
		t.Fatalf("SRem failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected SRem to return 1, got %d", n)
	}

	// Verify active_grids is now empty
	members, err := client.SMembers(ctx, activeGridsKey)
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected empty active_grids, got %v", members)
	}
}

func TestFireRegistration_FullFlow(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	fireKey := testKey("fire:37.576:126.977")
	activeGridsKey := testKey("active_grids")
	totalFiresKey := testKey("stats:total_fires")
	defer rdb.Del(ctx, fireKey, activeGridsKey, totalFiresKey)

	now := float64(time.Now().Unix())
	expireAt := now + 2400 // 40-minute TTL

	// Step 1: Register a new fire event (mirrors registerFireEvent in fire_ignite.go)
	eventID := "fire-abc123def456"
	err := client.ZAdd(ctx, fireKey, expireAt, eventID)
	if err != nil {
		t.Fatalf("ZAdd fire event failed: %v", err)
	}

	// Step 2: Track active grid
	err = client.SAdd(ctx, activeGridsKey, "37.576:126.977")
	if err != nil {
		t.Fatalf("SAdd active_grids failed: %v", err)
	}

	// Step 3: Increment fire counter
	err = client.Incr(ctx, totalFiresKey)
	if err != nil {
		t.Fatalf("Incr stats:total_fires failed: %v", err)
	}

	// Verify: count active fires
	nowStr := fmt.Sprintf("%f", now)
	activeCount, err := client.ZCount(ctx, fireKey, nowStr, "+inf")
	if err != nil {
		t.Fatalf("ZCount failed: %v", err)
	}
	if activeCount != 1 {
		t.Errorf("expected 1 active fire, got %d", activeCount)
	}

	// Verify: active grid is tracked
	grids, err := client.SMembers(ctx, activeGridsKey)
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}
	if len(grids) != 1 || grids[0] != "37.576:126.977" {
		t.Errorf("unexpected active_grids: %v", grids)
	}

	// Verify: total fires counter
	totalStr, err := client.Get(ctx, totalFiresKey)
	if err != nil {
		t.Fatalf("Get total_fires failed: %v", err)
	}
	if totalStr != "1" {
		t.Errorf("expected total_fires=1, got %s", totalStr)
	}

	// Add more fires to test stage progression counting
	for i := 0; i < 9; i++ {
		err := client.ZAdd(ctx, fireKey, expireAt+float64(i+1), fmt.Sprintf("fire-batch-%d", i))
		if err != nil {
			t.Fatalf("ZAdd batch failed: %v", err)
		}
	}

	// Verify total active count is now 10
	activeCount, err = client.ZCount(ctx, fireKey, nowStr, "+inf")
	if err != nil {
		t.Fatalf("ZCount after batch failed: %v", err)
	}
	if activeCount != 10 {
		t.Errorf("expected 10 active fires after batch, got %d", activeCount)
	}
}

func TestZIncrBy(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	key := testKey("stats:daily_ranking:2026-04-07")
	defer rdb.Del(ctx, key)

	// Simulate ranking increments
	err := client.ZIncrBy(ctx, key, 1, "광화문광장")
	if err != nil {
		t.Fatalf("ZIncrBy failed: %v", err)
	}
	err = client.ZIncrBy(ctx, key, 3, "강남역")
	if err != nil {
		t.Fatalf("ZIncrBy failed: %v", err)
	}
	err = client.ZIncrBy(ctx, key, 1, "광화문광장")
	if err != nil {
		t.Fatalf("ZIncrBy failed: %v", err)
	}

	// Verify ranking: 강남역 (3) > 광화문광장 (2)
	members, err := client.ZRevRangeWithScores(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRevRangeWithScores failed: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].Member != "강남역" || members[0].Score != 3 {
		t.Errorf("expected 강남역:3, got %s:%f", members[0].Member, members[0].Score)
	}
	if members[1].Member != "광화문광장" || members[1].Score != 2 {
		t.Errorf("expected 광화문광장:2, got %s:%f", members[1].Member, members[1].Score)
	}
}

func TestNewClient(t *testing.T) {
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClient(testRedisAddr(), "", 0)
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestClient_ScoreFormat_CompatibleWithPython(t *testing.T) {
	// This test verifies that the score format used in Go is compatible
	// with the Python server's use of float timestamps as sorted set scores.
	rdb := skipIfNoRedis(t)
	defer rdb.Close()

	client := NewClientFromRedis(rdb)
	ctx := context.Background()
	key := testKey("fire:compat:test")
	defer rdb.Del(ctx, key)

	// Python uses time.time() which returns float seconds.
	// Go uses time.Now().Unix() which returns int64 seconds.
	// Both are stored as float64 scores in Redis.
	now := float64(time.Now().Unix())
	expireAt := now + 2400.0

	err := client.ZAdd(ctx, key, expireAt, "fire-compat-1")
	if err != nil {
		t.Fatalf("ZAdd failed: %v", err)
	}

	// Query with string-formatted float (same as Python's str(time.time()))
	nowStr := fmt.Sprintf("%f", now)
	count, err := client.ZCount(ctx, key, nowStr, "+inf")
	if err != nil {
		t.Fatalf("ZCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d (score format mismatch)", count)
	}

	// Verify the score is retrievable and matches
	members, err := client.ZRevRangeWithScores(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ZRevRangeWithScores failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Score != expireAt {
		t.Errorf("expected score %f, got %f", expireAt, members[0].Score)
	}
}
