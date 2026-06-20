package options

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// sizeUnits maps rclone size suffixes to their byte multiplier. rclone treats
// k/M/G/… as powers of 1024 (the optional "i" is accepted and equivalent).
var sizeUnits = map[string]int64{
	"b": 1,
	"k": 1 << 10, "ki": 1 << 10,
	"m": 1 << 20, "mi": 1 << 20,
	"g": 1 << 30, "gi": 1 << 30,
	"t": 1 << 40, "ti": 1 << 40,
	"p": 1 << 50, "pi": 1 << 50,
}

// parseSize parses an rclone-style size into a byte count. "off" (or "-1")
// returns -1, meaning unlimited. A bare number is bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "off", "-1":
		return -1, nil
	case "":
		return 0, fmt.Errorf("empty size")
	}

	lower := strings.ToLower(s)
	// Split numeric prefix from unit suffix.
	i := 0
	for i < len(lower) && (lower[i] >= '0' && lower[i] <= '9' || lower[i] == '.') {
		i++
	}
	numPart, unitPart := lower[:i], lower[i:]
	if numPart == "" {
		return 0, fmt.Errorf("size %q has no number", s)
	}
	value, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}

	mult := int64(1)
	if unitPart != "" {
		m, ok := sizeUnits[unitPart]
		if !ok {
			return 0, fmt.Errorf("size %q has unknown unit %q", s, unitPart)
		}
		mult = m
	}
	return int64(value * float64(mult)), nil
}

// durationPattern matches rclone-style durations: one or more number+unit
// groups, e.g. "1s", "500ms", "1h30m".
var durationPattern = regexp.MustCompile(`^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h|d))+$`)

// looksLikeDuration reports whether s is a plausible rclone duration. rclone
// performs the authoritative parse; this catches obvious mistakes in the UI.
func looksLikeDuration(s string) bool {
	return durationPattern.MatchString(strings.TrimSpace(s))
}
