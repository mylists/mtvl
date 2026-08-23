package idgen

import (
	"strings"

	"github.com/google/uuid"
)

// New returns a random UUID string for shared category items.
func New() string {
	return uuid.NewString()
}

// Parse trims and validates a UUID. The second result is false when the value is not a UUID.
func Parse(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if _, err := uuid.Parse(s); err != nil {
		return "", false
	}
	return s, true
}
