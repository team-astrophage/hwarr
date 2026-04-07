package engine

// Redis key constants shared between fire progression and cleanup engines.
// These must match the Python server's fire key structure.
const (
	// FireKeyPrefix is the Redis key prefix for fire grid sorted sets.
	// Each grid's fires are stored in "fire:{gridId}" as a sorted set
	// where score = expiration timestamp and member = unique event ID.
	FireKeyPrefix = "fire:"

	// ActiveGridsKey is the Redis Set tracking all grid IDs with active fires.
	ActiveGridsKey = "active_grids"
)
