package main

import (
	"log"
	"net/url"
	"strconv"
	"strings"
)

// miscellaneous utility functions

func firstElementOf(s []string) string {
	// return first element of slice, or blank string if empty
	val := ""

	if len(s) > 0 {
		val = s[0]
	}

	return val
}

func sliceContainsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}

	return false
}

func sliceContainsValueFromSlice(haystack []string, needles []string) bool {
	for _, needle := range needles {
		if sliceContainsString(haystack, needle) {
			return true
		}
	}

	return false
}

func restrictValue(field string, val int, min int, fallback int) int {
	// default, if requested value isn't large enough
	res := fallback

	if val >= min {
		res = val
	} else {
		log.Printf(`value for "%s" is less than the minimum allowed value %d; defaulting to %d`, field, min, fallback)
	}

	return res
}

func nonemptyValues(val []string) []string {
	res := []string{}

	for _, s := range val {
		if s != "" {
			res = append(res, s)
		}
	}

	return res
}

func integerWithMinimum(str string, min int) int {
	val, err := strconv.Atoi(str)

	// fallback for invalid or nonsensical values
	if err != nil || val < min {
		val = min
	}

	return val
}

func titlesAreEqual(t1, t2 string) bool {
	// case-insensitive match.  titles must be nonempty
	var s1, s2 string

	if s1 = strings.Trim(t1, " "); s1 == "" {
		return false
	}

	if s2 = strings.Trim(t2, " "); s2 == "" {
		return false
	}

	return strings.EqualFold(s1, s2)
}

func isValidSortOrder(s string) bool {
	switch s {
	case "asc":
	case "desc":
	default:
		return false
	}

	return true
}

func isValidURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}
