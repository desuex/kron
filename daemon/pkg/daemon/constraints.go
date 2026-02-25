package daemon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"kron/core/pkg/core"
)

func applyConstraintModifier(spec *core.ConstraintSpec, modifierName, body string) error {
	parsed, err := parseConstraintSpec(body)
	if err != nil {
		return err
	}

	switch modifierName {
	case "only":
		spec.OnlyHours = mergeInts(spec.OnlyHours, parsed.hours)
		spec.OnlyDOW = mergeInts(spec.OnlyDOW, parsed.dow)
		spec.OnlyDOM = mergeInts(spec.OnlyDOM, parsed.dom)
		spec.OnlyMonths = mergeInts(spec.OnlyMonths, parsed.months)
		spec.OnlyBetween = append(spec.OnlyBetween, parsed.between...)
		spec.OnlyDates = append(spec.OnlyDates, parsed.dates...)
	case "avoid":
		spec.AvoidHours = mergeInts(spec.AvoidHours, parsed.hours)
		spec.AvoidDOW = mergeInts(spec.AvoidDOW, parsed.dow)
		spec.AvoidDOM = mergeInts(spec.AvoidDOM, parsed.dom)
		spec.AvoidMonths = mergeInts(spec.AvoidMonths, parsed.months)
		spec.AvoidBetween = append(spec.AvoidBetween, parsed.between...)
		spec.AvoidDates = append(spec.AvoidDates, parsed.dates...)
	default:
		return fmt.Errorf("unsupported constraint modifier %q", modifierName)
	}
	return nil
}

type parsedConstraintSpec struct {
	hours   []int
	dow     []int
	dom     []int
	months  []int
	between []core.TimeRange
	dates   []core.DateRange
}

func parseConstraintSpec(body string) (parsedConstraintSpec, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return parsedConstraintSpec{}, fmt.Errorf("constraint spec cannot be empty")
	}

	var out parsedConstraintSpec
	parts := strings.Split(body, ";")
	for _, rawClause := range parts {
		clause := strings.TrimSpace(rawClause)
		if clause == "" {
			continue
		}
		kv := strings.SplitN(clause, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" || strings.TrimSpace(kv[1]) == "" {
			return parsedConstraintSpec{}, fmt.Errorf("invalid constraint clause %q", clause)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		switch key {
		case "hours":
			hours, err := parseConstraintIntRangeSet(value, 0, 23, nil, false)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid hours clause: %w", err)
			}
			out.hours = mergeInts(out.hours, hours)
		case "dow":
			dow, err := parseConstraintIntRangeSet(value, 0, 6, dowAliases, true)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid dow clause: %w", err)
			}
			out.dow = mergeInts(out.dow, dow)
		case "dom":
			dom, err := parseConstraintIntRangeSet(value, 1, 31, nil, false)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid dom clause: %w", err)
			}
			out.dom = mergeInts(out.dom, dom)
		case "months":
			months, err := parseConstraintIntRangeSet(value, 1, 12, monthAliases, false)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid months clause: %w", err)
			}
			out.months = mergeInts(out.months, months)
		case "between":
			r, err := parseBetweenRange(value)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid between clause: %w", err)
			}
			out.between = append(out.between, r)
		case "date":
			day, err := parseDateYYYYMMDD(value)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid date clause: %w", err)
			}
			out.dates = append(out.dates, core.DateRange{StartDay: day, EndDay: day})
		case "dates":
			r, err := parseDateRangeYYYYMMDD(value)
			if err != nil {
				return parsedConstraintSpec{}, fmt.Errorf("invalid dates clause: %w", err)
			}
			out.dates = append(out.dates, r)
		default:
			return parsedConstraintSpec{}, fmt.Errorf("unknown constraint clause %q", key)
		}
	}

	if len(out.hours) == 0 && len(out.dow) == 0 && len(out.dom) == 0 &&
		len(out.months) == 0 && len(out.between) == 0 && len(out.dates) == 0 {
		return parsedConstraintSpec{}, fmt.Errorf("constraint spec cannot be empty")
	}

	return out, nil
}

func parseConstraintIntRangeSet(value string, min, max int, aliases map[string]int, allowSevenSunday bool) ([]int, error) {
	seen := map[int]bool{}
	for _, rawPart := range strings.Split(value, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return nil, fmt.Errorf("empty range element")
		}
		if strings.Contains(part, "-") {
			pair := strings.SplitN(part, "-", 2)
			if len(pair) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			start, err := parseConstraintValue(pair[0], aliases, allowSevenSunday)
			if err != nil {
				return nil, err
			}
			end, err := parseConstraintValue(pair[1], aliases, allowSevenSunday)
			if err != nil {
				return nil, err
			}
			if start < min || end > max || start > end {
				return nil, fmt.Errorf("range %q out of bounds %d-%d", part, min, max)
			}
			for v := start; v <= end; v++ {
				seen[v] = true
			}
			continue
		}

		v, err := parseConstraintValue(part, aliases, allowSevenSunday)
		if err != nil {
			return nil, err
		}
		if v < min || v > max {
			return nil, fmt.Errorf("value %q out of bounds %d-%d", part, min, max)
		}
		seen[v] = true
	}

	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out, nil
}

func parseConstraintValue(raw string, aliases map[string]int, allowSevenSunday bool) (int, error) {
	s := strings.ToUpper(strings.TrimSpace(raw))
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

func parseBetweenRange(value string) (core.TimeRange, error) {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return core.TimeRange{}, fmt.Errorf("expected HH:MM-HH:MM")
	}

	start, err := parseHHMM(parts[0])
	if err != nil {
		return core.TimeRange{}, err
	}
	end, err := parseHHMM(parts[1])
	if err != nil {
		return core.TimeRange{}, err
	}
	if start > end {
		return core.TimeRange{}, fmt.Errorf("start time must be <= end time")
	}
	return core.TimeRange{StartMinute: start, EndMinute: end}, nil
}

func parseDateYYYYMMDD(value string) (int, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid date %q", value)
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day(), nil
}

func parseDateRangeYYYYMMDD(value string) (core.DateRange, error) {
	parts := strings.SplitN(value, "..", 2)
	if len(parts) != 2 {
		return core.DateRange{}, fmt.Errorf("expected YYYY-MM-DD..YYYY-MM-DD")
	}

	start, err := parseDateYYYYMMDD(parts[0])
	if err != nil {
		return core.DateRange{}, err
	}
	end, err := parseDateYYYYMMDD(parts[1])
	if err != nil {
		return core.DateRange{}, err
	}
	if start > end {
		return core.DateRange{}, fmt.Errorf("start date must be <= end date")
	}
	return core.DateRange{StartDay: start, EndDay: end}, nil
}

func parseHHMM(raw string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid time %q", raw)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func mergeInts(existing, incoming []int) []int {
	seen := map[int]bool{}
	for _, v := range existing {
		seen[v] = true
	}
	for _, v := range incoming {
		seen[v] = true
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
