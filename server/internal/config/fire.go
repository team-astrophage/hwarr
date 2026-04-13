package config

// Fire-related application constants.
// All fire handlers (handler, sio) must reference these instead of local copies.
const (
	// FireTTLSec is the fire event TTL in seconds (12 hours).
	FireTTLSec = 43200

	// FireSpreadThreshold is the active fire count above which fires spread to neighbors.
	FireSpreadThreshold = 300

	// FireMaxCascadeDepth limits how many spread hops a single fire can cascade.
	FireMaxCascadeDepth = 8
)
