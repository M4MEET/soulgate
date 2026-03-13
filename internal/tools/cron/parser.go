package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseCronSchedule returns the next occurrence after `after` for a 5-field
// cron expression (minute hour dom month dow).
//
// Supported field syntax:
//   - *         — any value
//   - N         — exact value
//   - N-M       — inclusive range
//   - N,M,...   — list of values
//   - */N        — step (every N units, starting from the field's minimum)
//   - N-M/N      — step over a range
//
// The function advances minute-by-minute from `after+1m` and returns the
// first time that matches all five fields. It searches up to one year ahead
// before returning an error.
func ParseCronSchedule(expr string, after time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("cron: expression must have exactly 5 fields, got %d", len(fields))
	}

	minuteField := fields[0]
	hourField := fields[1]
	domField := fields[2] // day-of-month
	monthField := fields[3]
	dowField := fields[4] // day-of-week (0=Sunday, 6=Saturday)

	// Validate fields before entering the search loop so errors are caught
	// eagerly rather than after an expensive scan.
	if err := validateField(minuteField, 0, 59); err != nil {
		return time.Time{}, fmt.Errorf("cron: minute field: %w", err)
	}
	if err := validateField(hourField, 0, 23); err != nil {
		return time.Time{}, fmt.Errorf("cron: hour field: %w", err)
	}
	if err := validateField(domField, 1, 31); err != nil {
		return time.Time{}, fmt.Errorf("cron: dom field: %w", err)
	}
	if err := validateField(monthField, 1, 12); err != nil {
		return time.Time{}, fmt.Errorf("cron: month field: %w", err)
	}
	if err := validateField(dowField, 0, 6); err != nil {
		return time.Time{}, fmt.Errorf("cron: dow field: %w", err)
	}

	// Advance to the next whole minute (discard seconds/nanoseconds).
	t := after.Truncate(time.Minute).Add(time.Minute)

	limit := after.Add(366 * 24 * time.Hour)
	for !t.After(limit) {
		minuteOK, _ := matchField(minuteField, t.Minute(), 0, 59)
		hourOK, _ := matchField(hourField, t.Hour(), 0, 23)
		domOK, _ := matchField(domField, t.Day(), 1, 31)
		monthOK, _ := matchField(monthField, int(t.Month()), 1, 12)
		// time.Weekday: 0=Sunday … 6=Saturday, matching cron convention.
		dowOK, _ := matchField(dowField, int(t.Weekday()), 0, 6)

		if minuteOK && hourOK && domOK && monthOK && dowOK {
			return t, nil
		}

		t = t.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("cron: no occurrence found within one year for expression %q", expr)
}

// validateField checks that a cron field string is syntactically valid for
// the given [min, max] range. Returns nil when valid.
func validateField(field string, min, max int) error {
	_, err := matchField(field, min, min, max)
	return err
}

// matchField reports whether value matches the cron field spec.
// min and max define the legal bounds for the field.
func matchField(field string, value, min, max int) (bool, error) {
	// List: "1,3,5"
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, p := range parts {
			ok, err := matchSingle(p, value, min, max)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	return matchSingle(field, value, min, max)
}

// matchSingle handles a single cron field segment (no commas): *, N, N-M,
// */step, or N-M/step.
func matchSingle(segment string, value, min, max int) (bool, error) {
	// Step: "*/N" or "N-M/N"
	if strings.Contains(segment, "/") {
		parts := strings.SplitN(segment, "/", 2)
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return false, fmt.Errorf("invalid step in %q", segment)
		}

		var rangeMin, rangeMax int
		if parts[0] == "*" {
			rangeMin, rangeMax = min, max
		} else if strings.Contains(parts[0], "-") {
			rangeMin, rangeMax, err = parseRange(parts[0], min, max)
			if err != nil {
				return false, err
			}
		} else {
			rangeMin, err = parseInt(parts[0], min, max)
			if err != nil {
				return false, err
			}
			rangeMax = max
		}

		if value < rangeMin || value > rangeMax {
			return false, nil
		}
		return (value-rangeMin)%step == 0, nil
	}

	// Wildcard: "*"
	if segment == "*" {
		return true, nil
	}

	// Range: "N-M"
	if strings.Contains(segment, "-") {
		lo, hi, err := parseRange(segment, min, max)
		if err != nil {
			return false, err
		}
		return value >= lo && value <= hi, nil
	}

	// Exact: "N"
	n, err := parseInt(segment, min, max)
	if err != nil {
		return false, err
	}
	return value == n, nil
}

func parseRange(s string, min, max int) (int, int, error) {
	parts := strings.SplitN(s, "-", 2)
	lo, err := parseInt(parts[0], min, max)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range lo in %q: %w", s, err)
	}
	hi, err := parseInt(parts[1], min, max)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range hi in %q: %w", s, err)
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("range lo %d > hi %d in %q", lo, hi, s)
	}
	return lo, hi, nil
}

func parseInt(s string, min, max int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", s)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("value %d out of range [%d, %d]", n, min, max)
	}
	return n, nil
}

// ParseDuration parses human-friendly duration strings, extending
// time.ParseDuration with a "d" suffix for days.
//
// Supported suffixes: s, m, h, d.
// Examples: "30s", "5m", "1h", "1d", "2h30m".
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("duration: empty string")
	}

	// Handle "d" suffix for days before delegating to time.ParseDuration.
	// We convert Nd → N*24h so that standard parsing handles the rest.
	processed, err := expandDays(s)
	if err != nil {
		return 0, fmt.Errorf("duration: %w", err)
	}

	d, err := time.ParseDuration(processed)
	if err != nil {
		return 0, fmt.Errorf("duration: cannot parse %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration: must be positive, got %q", s)
	}
	return d, nil
}

// expandDays replaces Nd tokens with their equivalent number of hours so
// time.ParseDuration can handle the resulting string.
// e.g. "1d" → "24h", "2d12h" → "48h12h".
func expandDays(s string) (string, error) {
	// Find 'd' characters and expand them.
	var result strings.Builder
	i := 0
	for i < len(s) {
		// Collect a numeric prefix.
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == 'd' {
			// We have a day component.
			if j == i {
				return "", fmt.Errorf("'d' suffix without a numeric value")
			}
			days, err := strconv.Atoi(s[i:j])
			if err != nil {
				return "", fmt.Errorf("invalid day value %q", s[i:j])
			}
			result.WriteString(strconv.Itoa(days*24) + "h")
			i = j + 1
		} else {
			// Not a day segment — copy as-is until the next digit or end.
			if j == i {
				// Current char is non-numeric and not 'd'.
				result.WriteByte(s[i])
				i++
			} else {
				result.WriteString(s[i:j])
				i = j
			}
		}
	}
	return result.String(), nil
}
