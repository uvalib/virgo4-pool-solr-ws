package main

import (
	"fmt"
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

func sliceContainsString(haystack []string, needle string, insensitive bool) bool {
	if len(haystack) == 0 {
		return false
	}

	for _, item := range haystack {
		a := item
		b := needle

		if insensitive == true {
			a = strings.ToLower(item)
			b = strings.ToLower(needle)
		}

		if a == b {
			return true
		}
	}

	return false
}

func sliceContainsAnyValueFromSlice(haystack []string, needles []string, insensitive bool) bool {
	if len(haystack) == 0 || len(needles) == 0 {
		return false
	}

	for _, needle := range needles {
		if sliceContainsString(haystack, needle, insensitive) == true {
			return true
		}
	}

	return false
}

func sliceContainsAllValuesFromSlice(haystack []string, needles []string, insensitive bool) bool {
	if len(haystack) == 0 || len(needles) == 0 {
		return false
	}

	for _, needle := range needles {
		if sliceContainsString(haystack, needle, insensitive) == false {
			return false
		}
	}

	return true
}

func slicesAreEqual(haystack []string, needles []string, insensitive bool) bool {
	if sliceContainsAllValuesFromSlice(haystack, needles, insensitive) == false {
		return false
	}

	if len(haystack) != len(needles) {
		return false
	}

	return true
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

func uniqueStrings(s []string) []string {
	var uniq []string

	seen := make(map[string]bool)

	for _, val := range s {
		key := strings.ToLower(val)

		if seen[key] == false {
			uniq = append(uniq, val)
			seen[key] = true
		}
	}

	return uniq
}

func chunkStrings(list []string, size int) [][]string {
	var chunks [][]string

	for size < len(list) {
		list, chunks = list[size:], append(chunks, list[0:size:size])
	}

	if len(list) > 0 {
		chunks = append(chunks, list)
	}

	return chunks
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}

func hasAnySuffix(s string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}

	return false
}

func (s *searchContext) getExternalSolrValue(field string, internalValue string) (string, error) {
	extMap := s.pool.maps.solrExternalValues[field]

	if extMap == nil {
		return internalValue, nil
	}

	xid, ok := extMap[internalValue]

	if ok == false {
		return "", fmt.Errorf("solr field: [%s]  ignoring unmapped internal value: [%s]", field, internalValue)
	}

	if xid == "" {
		return "", fmt.Errorf("solr field: [%s]  ignoring empty internal value: [%s]", field, internalValue)
	}

	return s.client.localize(xid), nil
}

func (s *searchContext) getInternalSolrValue(field string, externalValue string) (string, error) {
	intMap := s.pool.maps.solrInternalValues[field]

	if intMap == nil {
		return externalValue, nil
	}

	val, ok := intMap[externalValue]

	if ok == false {
		return externalValue, fmt.Errorf("solr field: [%s]  ignoring unmapped external value: [%s]", field, externalValue)
	}

	if val == "" {
		return externalValue, fmt.Errorf("solr field: [%s]  ignoring empty external value: [%s]", field, externalValue)
	}

	return val, nil
}
