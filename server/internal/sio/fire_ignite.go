package sio

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/homepy/hwarr/server/internal/config"
	"github.com/homepy/hwarr/server/internal/geodata"
	"github.com/homepy/hwarr/server/internal/grid"
	"github.com/homepy/hwarr/server/internal/model"
	"github.com/homepy/hwarr/server/internal/ranking"
	socketio "github.com/homeworldio/socketio-go"
)

// StatsDailyFiresTTL is the TTL for daily fire counters (48h for KST date boundary safety).
const StatsDailyFiresTTL = 48 * time.Hour

// RedisFireWriter abstracts the Redis operations needed to register fire events.
type RedisFireWriter interface {
	RedisFireCounter // embeds ZCount
	// ZAdd adds a member with score to a sorted set.
	ZAdd(ctx context.Context, key string, score float64, member string) error
	// SAdd adds a member to a set.
	SAdd(ctx context.Context, key string, member string) error
	// Incr increments a key by 1.
	Incr(ctx context.Context, key string) error
	// Expire sets a timeout on key.
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// ZIncrBy increments a sorted set member's score.
	ZIncrBy(ctx context.Context, key string, increment float64, member string) error
}

// fireIgniteData is the client payload for fire:ignite.
type fireIgniteData struct {
	Lat  *float64 `json:"lat,omitempty"`
	Lng  *float64 `json:"lng,omitempty"`
	Demo bool     `json:"demo,omitempty"`
}

// fireRegistration holds the result of a fire registration in Redis.
type fireRegistration struct {
	GridID      string
	ActiveCount int
	SpreadPath  [][2]string // pairs of (from, to) grid IDs
}

// demoLocation mirrors the predefined location structure from handler package.
type demoLocation struct {
	ID          string
	Name        string
	Lat         float64
	Lng         float64
	Description string
}

// predefinedDemoLocations matches server/routes/map_config.py _PREDEFINED_LOCATIONS.
// Duplicated here to avoid circular import with handler package.
var predefinedDemoLocations = []demoLocation{
	{ID: "gwanghwamun", Name: "광화문광장", Lat: 37.5760, Lng: 126.9769, Description: "서울 광화문광장"},
	{ID: "gangnam", Name: "강남역", Lat: 37.4979, Lng: 127.0276, Description: "서울 강남역 사거리"},
	{ID: "hongdae", Name: "홍대입구", Lat: 37.5563, Lng: 126.9236, Description: "서울 홍대입구역"},
	{ID: "yeouido", Name: "여의도공원", Lat: 37.5284, Lng: 126.9344, Description: "서울 여의도공원"},
	{ID: "jamsil", Name: "잠실종합운동장", Lat: 37.5153, Lng: 127.0728, Description: "서울 잠실종합운동장"},
	{ID: "namsan", Name: "남산타워", Lat: 37.5512, Lng: 126.9882, Description: "서울 남산서울타워"},
	{ID: "itaewon", Name: "이태원", Lat: 37.5345, Lng: 126.9946, Description: "서울 이태원거리"},
	{ID: "busan_haeundae", Name: "해운대해수욕장", Lat: 35.1587, Lng: 129.1604, Description: "부산 해운대해수욕장"},
	{ID: "daegu_dongseong", Name: "동성로", Lat: 35.8691, Lng: 128.5958, Description: "대구 동성로"},
	{ID: "incheon_songdo", Name: "송도센트럴파크", Lat: 37.3925, Lng: 126.6632, Description: "인천 송도센트럴파크"},
	{ID: "gwangju_chungjang", Name: "충장로", Lat: 35.1488, Lng: 126.9156, Description: "광주 충장로"},
	{ID: "daejeon_dunsan", Name: "둔산동", Lat: 36.3511, Lng: 127.3782, Description: "대전 둔산동"},
	{ID: "jeju_hallasan", Name: "한라산", Lat: 33.3617, Lng: 126.5292, Description: "제주 한라산"},
	{ID: "suwon_hwaseong", Name: "수원화성", Lat: 37.2870, Lng: 127.0095, Description: "수원 화성행궁"},
	{ID: "gyeongju_bulguksa", Name: "불국사", Lat: 35.7900, Lng: 129.3322, Description: "경주 불국사"},
	{ID: "coex", Name: "코엑스", Lat: 37.5126, Lng: 127.0590, Description: "서울 코엑스 (해커톤 데모 장소)"},
}

// RegisterFireIgniteHandler registers the "fire:ignite" Socket.IO event.
//
// Client sends: { "lat": float, "lng": float, "demo": bool }
// Server:
//  1. Converts GPS to grid ID (or picks random demo location if demo=true without coords)
//  2. Registers fire event in Redis (with neighbor spreading if threshold exceeded)
//  3. Broadcasts fire:ignite to grid room (viewport subscribers)
//  4. Enqueues fire:update to batcher (room-scoped, replaces global broadcast)
//  5. Returns ack with full fire state to the igniting client
//
// Mirrors Python server/sio/events.py handle_fire_ignite.
func RegisterFireIgniteHandler(
	sioServer *socketio.Server,
	manager *ConnectionManager,
	redis RedisFireWriter,
	resolver *geodata.AdminRegionResolver,
	logger *log.Logger,
	batcher *FireBatcher,
) {
	if logger == nil {
		logger = log.Default()
	}

	sioServer.On("fire:ignite", func(sid string, args ...json.RawMessage) ([]interface{}, error) {
		// Parse client payload
		if len(args) == 0 || len(args[0]) == 0 {
			return []interface{}{map[string]interface{}{
				"error": "lat and lng are required",
			}}, nil
		}

		var data fireIgniteData
		if err := json.Unmarshal(args[0], &data); err != nil {
			logger.Printf("fire:ignite from %s invalid JSON: %v", sid, err)
			return []interface{}{map[string]interface{}{
				"error": "lat and lng are required",
			}}, nil
		}

		lat := data.Lat
		lng := data.Lng
		demo := data.Demo
		var demoLocationName string

		// Demo mode fallback: pick a random predefined location when GPS
		// is unavailable (client sends { "demo": true } without lat/lng)
		if demo && (lat == nil || lng == nil) {
			loc := predefinedDemoLocations[rand.Intn(len(predefinedDemoLocations))]
			lat = &loc.Lat
			lng = &loc.Lng
			demoLocationName = loc.Name
			logger.Printf("fire:ignite demo fallback sid=%s -> %s (%s)", sid, loc.ID, loc.Name)
		} else if lat == nil || lng == nil {
			logger.Printf("fire:ignite from %s missing lat/lng", sid)
			return []interface{}{map[string]interface{}{
				"error": "lat and lng are required",
			}}, nil
		}

		// GPS -> grid ID
		requestedGridID := grid.ToGridID(*lat, *lng)

		// Generate unique event ID
		eventID := fmt.Sprintf("fire-%s", randomHexSIO(12))
		expireAt := float64(time.Now().Unix()) + config.FireTTLSec

		// Register fire in Redis (may spread to neighbor if threshold exceeded)
		ctx := context.Background()
		reg, err := registerFireEvent(ctx, redis, requestedGridID, eventID, expireAt, resolver, logger)
		if err != nil {
			logger.Printf("ERROR fire:ignite from %s Redis error (grid=%s): %v", sid, requestedGridID, err)
			return []interface{}{map[string]interface{}{
				"error": "failed to register fire event",
			}}, nil
		}

		gridID := reg.GridID
		activeCount := reg.ActiveCount

		// Compute center lat/lng for the landing grid (source of truth).
		centerLat, centerLng, cerr := grid.GridIDToCenter(gridID)
		if cerr != nil {
			logger.Printf("fire:ignite GridIDToCenter error for %s: %v", gridID, cerr)
			centerLat = *lat
			centerLng = *lng
		}

		// Build spread path with lat/lng per segment.
		spreadPath := make([]map[string]interface{}, len(reg.SpreadPath))
		for i, sp := range reg.SpreadPath {
			fromLat, fromLng, _ := grid.GridIDToCenter(sp[0])
			toLat, toLng, _ := grid.GridIDToCenter(sp[1])
			spreadPath[i] = map[string]interface{}{
				"from":    sp[0],
				"to":      sp[1],
				"fromLat": fromLat,
				"fromLng": fromLng,
				"toLat":   toLat,
				"toLng":   toLng,
			}
		}

		// Build grid state with stage info
		state := model.BuildGridState(gridID, activeCount, centerLat, centerLng)

		// Broadcast fire:ignite room-scoped (viewport subscribers only)
		now := float64(time.Now().UnixMilli()) / 1000.0
		ignitePayload := map[string]interface{}{
			"gridId":      gridID,
			"eventId":     eventID,
			"lat":         centerLat,
			"lng":         centerLng,
			"activeCount": activeCount,
			"stage":       state.Stage,
			"stageInfo":   state.StageInfo,
			"ignitedBy":   sid,
			"timestamp":   now,
		}

		if _, err := sioServer.BroadcastToRoom("/", gridID, "fire:ignite", ignitePayload); err != nil {
			logger.Printf("fire:ignite room broadcast failed for %s: %v", gridID, err)
		}

		// Enqueue batched room-scoped update (fire:update)
		batcher.Add(FireUpdate{
			GridID:      gridID,
			ActiveCount: activeCount,
			Stage:       state.Stage,
			EventID:     eventID,
			Timestamp:   now,
			Lat:         centerLat,
			Lng:         centerLng,
		})

		// Broadcast fire:spread animation to all affected grid rooms (de-duped).
		if len(spreadPath) > 0 {
			spreadPayload := map[string]interface{}{
				"path":      spreadPath,
				"eventId":   eventID,
				"timestamp": now,
			}
			affectedGrids := make(map[string]struct{})
			for _, sp := range reg.SpreadPath {
				affectedGrids[sp[0]] = struct{}{}
				affectedGrids[sp[1]] = struct{}{}
			}
			for room := range affectedGrids {
				if _, err := sioServer.BroadcastToRoom("/", room, "fire:spread", spreadPayload); err != nil {
					logger.Printf("fire:spread room broadcast failed for %s: %v", room, err)
				}
			}
		}

		logger.Printf("fire:ignite sid=%s requested=%s landed=%s count=%d stage=%d spread=%d",
			sid, requestedGridID, gridID, activeCount, state.Stage, len(spreadPath))

		// Build ack response to the igniting client
		result := map[string]interface{}{
			"status":          "ok",
			"gridId":          gridID,
			"requestedGridId": requestedGridID,
			"eventId":         eventID,
			"lat":             centerLat,
			"lng":             centerLng,
			"activeCount":     activeCount,
			"stage":           state.Stage,
			"stageInfo":       state.StageInfo,
			"spreadPath":      spreadPath,
		}

		// Enrich response with demo-specific fields
		if demo {
			result["demo"] = true
			if demoLocationName != "" {
				result["demoLocation"] = demoLocationName
			}
		}

		return []interface{}{result}, nil
	})
}

// registerFireEvent registers a fire event in Redis, spreading to neighbors if needed.
// Returns the landing grid ID, active count, and spread path.
// Mirrors Python FireProgressionEngine.register_fire().
func registerFireEvent(
	ctx context.Context,
	redis RedisFireWriter,
	gridID, eventID string,
	expireAt float64,
	resolver *geodata.AdminRegionResolver,
	logger *log.Logger,
) (*fireRegistration, error) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	currentGrid := gridID
	var spreadPath [][2]string

	// Walk the cascade: spread to neighbor if current grid is at/above threshold
	for i := 0; i < config.FireMaxCascadeDepth; i++ {
		key := fmt.Sprintf("fire:%s", currentGrid)
		count, err := redis.ZCount(ctx, key, now, "+inf")
		if err != nil {
			return nil, fmt.Errorf("ZCount for %s: %w", key, err)
		}
		if count < config.FireSpreadThreshold {
			break // capacity available — land here
		}
		// Spread to a random neighbor
		neighbors, err := grid.GetNeighbors8(currentGrid)
		if err != nil {
			break
		}
		nextGrid := neighbors[rand.Intn(len(neighbors))]
		spreadPath = append(spreadPath, [2]string{currentGrid, nextGrid})
		currentGrid = nextGrid
	}

	landingKey := fmt.Sprintf("fire:%s", currentGrid)

	// Add fire event to the landing grid's sorted set
	if err := redis.ZAdd(ctx, landingKey, expireAt, eventID); err != nil {
		return nil, fmt.Errorf("ZAdd for %s: %w", landingKey, err)
	}

	// Track active grid
	if err := redis.SAdd(ctx, "active_grids", currentGrid); err != nil {
		return nil, fmt.Errorf("SAdd active_grids: %w", err)
	}

	// Increment total fire counter
	if err := redis.Incr(ctx, "stats:total_fires"); err != nil {
		return nil, fmt.Errorf("Incr stats:total_fires: %w", err)
	}

	// Increment today's fire counter (KST = UTC+9)
	kst := time.FixedZone("KST", 9*60*60)
	todayStr := time.Now().In(kst).Format("2006-01-02")
	todayKey := fmt.Sprintf("stats:daily_fires:%s", todayStr)
	if err := redis.Incr(ctx, todayKey); err != nil {
		return nil, fmt.Errorf("Incr %s: %w", todayKey, err)
	}
	// 48h TTL for KST date boundary safety (matches Python STATS_DAILY_FIRES_TTL_SEC)
	_ = redis.Expire(ctx, todayKey, StatsDailyFiresTTL)

	// Increment daily ranking for the location region
	rankingKey := fmt.Sprintf("stats:daily_ranking:%s", todayStr)
	member := ranking.ResolveMember(currentGrid, resolver)
	_ = redis.ZIncrBy(ctx, rankingKey, 1, member)

	// Get final active count
	activeCount, err := redis.ZCount(ctx, landingKey, now, "+inf")
	if err != nil {
		return nil, fmt.Errorf("ZCount final for %s: %w", landingKey, err)
	}

	return &fireRegistration{
		GridID:      currentGrid,
		ActiveCount: int(activeCount),
		SpreadPath:  spreadPath,
	}, nil
}

// randomHexSIO generates a random hex string of the given length.
// Named with SIO suffix to avoid collision with handler package's randomHex.
func randomHexSIO(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(b)
}
