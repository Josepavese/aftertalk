package api

import (
	"errors"
	"sync"
	"time"
)

// roomTTL is how long a room entry is kept in the cache after creation.
// After this period the room code can be reused for a new session.
const roomTTL = 3 * time.Hour

// roomEntry holds the tokens and metadata for one active room.
type roomEntry struct {
	tokens    map[string]string // role/key → token
	issued    map[string]bool   // role → already issued
	names     map[string]string // role → participant name (for reconnection)
	createdAt time.Time
}

// roomCache maps a room code to the pre-generated tokens for each role.
// All operations are atomic to prevent race conditions when two browsers
// join the same room simultaneously.
type roomCache struct {
	rooms map[string]*roomEntry
	mu    sync.Mutex
}

var errRoleTaken = errors.New("role already taken in this room")

// getOrCreate atomically returns the token for the given role in the room,
// creating the session (via create()) if the room does not yet exist or has expired.
// If the role is already issued but the name matches (reconnection), the existing
// token is re-issued. Returns errRoleTaken if the role is taken by a different name.
func (rc *roomCache) getOrCreate(code, role, name string, create func() (map[string]string, error)) (string, string, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Expire stale entries so the same room code can be reused.
	if entry, ok := rc.rooms[code]; ok && time.Since(entry.createdAt) < roomTTL {
		if entry.issued[role] {
			// Allow reconnection if the name matches.
			if entry.names[role] == name {
				return entry.tokens["_session_id"], entry.tokens[role], nil
			}
			return "", "", errRoleTaken
		}
		entry.issued[role] = true
		entry.names[role] = name
		return entry.tokens["_session_id"], entry.tokens[role], nil
	}

	// Room is new or expired — create a fresh session.
	tokens, err := create()
	if err != nil {
		return "", "", err
	}
	rc.rooms[code] = &roomEntry{
		tokens:    tokens,
		issued:    map[string]bool{role: true},
		names:     map[string]string{role: name},
		createdAt: time.Now(),
	}
	return tokens["_session_id"], tokens[role], nil
}
