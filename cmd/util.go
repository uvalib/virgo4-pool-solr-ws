package main

import (
	"fmt"
	"log"
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

func timeoutWithMinimum(str string, min int) int {
	val, err := strconv.Atoi(str)

	// fallback for invalid or nonsensical timeout values
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

func getIIIFBaseURL(doc *solrDocument, field string) string {
	// FIXME: remove after iiif_image_url is correct
	// construct iiif image base url from known image identifier prefixes.
	// this fallback url conveniently points to an "orginial image missing" image
	baseURL := "https://iiif.lib.virginia.edu/iiif/uva-lib:1043352"

	identifierField := doc.getStringSliceValueByTag(field)

	for _, item := range identifierField {
		if strings.HasPrefix(item, "tsm:") || strings.HasPrefix(item, "uva-lib:") {
			baseURL = fmt.Sprintf("https://iiif.lib.virginia.edu/iiif/%s", item)
			break
		}
	}

	return baseURL
}
