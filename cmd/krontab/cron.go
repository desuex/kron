package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSpec struct {
	minutes  [60]bool
	hours    [24]bool
	dom      [32]bool
	months   [13]bool
	dow      [7]bool
	domAny   bool
	dowAny   bool
	location *time.Location
}

func parseCronSpec(fields [5]string, tz string) (CronSpec, error) {
	loc := time.UTC
	if tz != "" && tz != "UTC" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return CronSpec{}, fmt.Errorf("invalid timezone %q", tz)
		}
		loc = loaded
	}

	minutes, _, err := parseField(fields[0], 0, 59, nil, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, _, err := parseField(fields[1], 0, 23, nil, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid hour field: %w", err)
	}
	dom, domAny, err := parseField(fields[2], 1, 31, nil, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	months, _, err := parseField(fields[3], 1, 12, monthAliases, false)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid month field: %w", err)
	}
	dow, dowAny, err := parseField(fields[4], 0, 6, dowAliases, true)
	if err != nil {
		return CronSpec{}, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	var minuteArr [60]bool
	var hourArr [24]bool
	var domArr [32]bool
	var monthArr [13]bool
	var dowArr [7]bool
	for v := range minutes {
		minuteArr[v] = true
	}
	for v := range hours {
		hourArr[v] = true
	}
	for v := range dom {
		domArr[v] = true
	}
	for v := range months {
		monthArr[v] = true
	}
	for v := range dow {
		dowArr[v] = true
	}

	return CronSpec{
		minutes:  minuteArr,
		hours:    hourArr,
		dom:      domArr,
		months:   monthArr,
		dow:      dowArr,
		domAny:   domAny,
		dowAny:   dowAny,
		location: loc,
	}, nil
}

func (c CronSpec) NextAfter(after time.Time) (time.Time, error) {
	candidate := after.In(c.location).Truncate(time.Minute).Add(time.Minute)
	limit := candidate.AddDate(10, 0, 0)

	for !candidate.After(limit) {
		if c.matches(candidate) {
			return time.Date(
				candidate.Year(),
				candidate.Month(),
				candidate.Day(),
				candidate.Hour(),
				candidate.Minute(),
				0,
				0,
				c.location,
			).UTC(), nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 10 years")
}

func (c CronSpec) NextN(after time.Time, count int) ([]time.Time, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be > 0")
	}

	out := make([]time.Time, 0, count)
	cursor := after
	for i := 0; i < count; i++ {
		next, err := c.NextAfter(cursor)
		if err != nil {
			return nil, err
		}
		out = append(out, next)
		cursor = next
	}
	return out, nil
}

func (c CronSpec) matches(t time.Time) bool {
	if !c.months[int(t.Month())] {
		return false
	}
	if !c.hours[t.Hour()] {
		return false
	}
	if !c.minutes[t.Minute()] {
		return false
	}

	domMatch := c.dom[t.Day()]
	dowMatch := c.dow[int(t.Weekday())]

	if c.domAny && c.dowAny {
		return true
	}
	if c.domAny {
		return dowMatch
	}
	if c.dowAny {
		return domMatch
	}
	return domMatch || dowMatch
}

var monthAliases = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowAliases = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

func parseField(field string, min, max int, aliases map[string]int, allowSevenSunday bool) (map[int]bool, bool, error) { // NOSONAR
	out := make(map[int]bool)
	if field == "*" {
		for i := min; i <= max; i++ {
			out[i] = true
		}
		return out, true, nil
	}

	for _, part := range strings.Split(field, ",") {
		step := 1
		base := part
		if strings.Contains(part, "/") {
			parts := strings.SplitN(part, "/", 2)
			if len(parts) != 2 {
				return nil, false, fmt.Errorf("invalid step expression %q", part)
			}
			base = parts[0]
			s, err := strconv.Atoi(parts[1])
			if err != nil || s <= 0 {
				return nil, false, fmt.Errorf("invalid step value %q", parts[1])
			}
			step = s
		}

		start := min
		end := max
		if base != "*" {
			if strings.Contains(base, "-") {
				rangeParts := strings.SplitN(base, "-", 2)
				if len(rangeParts) != 2 {
					return nil, false, fmt.Errorf("invalid range %q", base)
				}
				sv, err := parseCronValue(rangeParts[0], aliases, allowSevenSunday)
				if err != nil {
					return nil, false, err
				}
				ev, err := parseCronValue(rangeParts[1], aliases, allowSevenSunday)
				if err != nil {
					return nil, false, err
				}
				start, end = sv, ev
			} else {
				v, err := parseCronValue(base, aliases, allowSevenSunday)
				if err != nil {
					return nil, false, err
				}
				start, end = v, v
			}
		}

		if start < min || end > max || start > end {
			return nil, false, fmt.Errorf("value out of range %d-%d", min, max)
		}
		for v := start; v <= end; v += step {
			out[v] = true
		}
	}

	return out, false, nil
}

func parseCronValue(raw string, aliases map[string]int, allowSevenSunday bool) (int, error) {
	s := strings.ToUpper(raw)
	if aliases != nil {
		if v, ok := aliases[s]; ok {
			return v, nil
		}
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", raw)
	}
	if allowSevenSunday && v == 7 {
		return 0, nil
	}
	return v, nil
}
