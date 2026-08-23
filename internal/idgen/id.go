package idgen

import (
	"strings"

	"github.com/google/uuid"
)

// New returns a random UUID string for users and shared category items.
func New() string {
	return uuid.NewString()
}

// Parse trims and validates a UUID. The second result is false when the value is not a UUID.
func Parse(s string) (string, bool) {
	id, ok := ParseUUID(s)
	if !ok {
		return "", false
	}
	return id.String(), true
}

// ParseUUID trims and validates a UUID.
func ParseUUID(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// Arg returns a uuid.UUID so Postgres binds UUID columns as uuid, not integer or text.
func Arg(s string) any {
	if id, ok := ParseUUID(s); ok {
		return id
	}
	return strings.TrimSpace(s)
}

// Args converts UUID strings for IN (?) clauses.
func Args(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = Arg(id)
	}
	return out
}
