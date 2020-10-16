package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode"
)

// titleization logic

type titleizeConfig struct {
	debug           bool
	wordDelimiters  string
	partDelimiters  string
	mixedCaseWords  []string
	upperCaseWords  []string
	lowerCaseWords  []string
	multiPartWords  []string
	ordinalPatterns []string
}

type titleizeREs struct {
	wordSeparator    *regexp.Regexp
	partSeparator    *regexp.Regexp
	partExtractor    *regexp.Regexp
	mixedCaseWords   *regexp.Regexp
	upperCaseWords   *regexp.Regexp
	lowerCaseWords   *regexp.Regexp
	multiPartWords   *regexp.Regexp
	ordinalPatterns  *regexp.Regexp
	inseparableParts *regexp.Regexp
	inseparableWords *regexp.Regexp
	areInitials      *regexp.Regexp
	hasUpperCase     *regexp.Regexp
	hasLowerCase     *regexp.Regexp
	allUpperCase     *regexp.Regexp
	allLowerCase     *regexp.Regexp
	isCapitalizable  *regexp.Regexp
}

type titleizeContext struct {
	re           titleizeREs
	placeHolder  string
	mixedCaseMap map[string]string
	debug        bool
}

func newTitleizeContext(cfg *titleizeConfig) *titleizeContext {
	t := titleizeContext{
		debug:       cfg.debug,
		placeHolder: `}}}!!!{{{`,
	}

	// ensure sane defaults for missing values

	wordDelimiters := `[:space:]`
	if len(cfg.wordDelimiters) > 0 {
		wordDelimiters = cfg.wordDelimiters
	}

	partDelimiters := `:;/+-`
	if len(cfg.partDelimiters) > 0 {
		partDelimiters = cfg.partDelimiters
	}

	// set up word/part regexps

	t.re.wordSeparator = t.newRegex(fmt.Sprintf("[%s]+", wordDelimiters))
	t.re.partSeparator = t.newRegex(fmt.Sprintf("[%s]+", partDelimiters))
	t.re.partExtractor = t.newRegex(fmt.Sprintf("[^%s]+", partDelimiters))

	// within all configured word lists, find:
	// * strings containing word delimiter(s) that should be treated as a unit
	// * strings containing part delimiter(s) that should be treated as a unit

	var allWords []string
	allWords = append(allWords, cfg.mixedCaseWords...)
	allWords = append(allWords, cfg.upperCaseWords...)
	allWords = append(allWords, cfg.lowerCaseWords...)
	allWords = append(allWords, cfg.multiPartWords...)

	var inseparableWords []string
	var inseparableParts []string

	for _, str := range allWords {
		if t.re.wordSeparator.MatchString(str) {
			inseparableWords = append(inseparableWords, str)
		}

		if t.re.partSeparator.MatchString(str) {
			inseparableParts = append(inseparableParts, str)
		}
	}

	// create mapping to restore case of mixed-case words

	t.mixedCaseMap = make(map[string]string)
	for _, word := range cfg.mixedCaseWords {
		t.mixedCaseMap[strings.ToLower(word)] = word
	}

	// non-configureable regexps

	// matches strings containing upper- or lower-case letters
	t.re.hasUpperCase = t.newRegex(`.*[[:upper:]].*`)
	t.re.hasLowerCase = t.newRegex(`.*[[:lower:]].*`)

	// matches strings that begin with an upper- or lower-case letter,
	// and does not contain any characters of the opposite case
	t.re.allUpperCase = t.newRegex(`^[[:upper:]][^[:lower:]]*$`)
	t.re.allLowerCase = t.newRegex(`^[[:lower:]][^[:upper:]]*$`)

	// matches strings that are either:
	// * a single lower-case letter, or
	// * start with two lower-case letters, or
	// * start with a lower-case letter followed by an apostrophe and a lower-case letter
	t.re.isCapitalizable = t.newRegex(`^[[:lower:]]($|('|)[[:lower:]])`)

	// matches strings that are composed entirely of alternating alphabetical/period characters
	t.re.areInitials = t.newRegex(t.makeCaseInsensitiveWordPattern([]string{`(?:[[:alpha:]]\.)+`}))

	// matches strings in each configured word list

	neverMatch := t.newRegex(`^\b$`)

	t.re.mixedCaseWords = neverMatch
	if len(cfg.mixedCaseWords) > 0 {
		t.re.mixedCaseWords = t.newRegex(t.makeCaseInsensitiveWordPattern(cfg.mixedCaseWords))
	}

	t.re.upperCaseWords = neverMatch
	if len(cfg.upperCaseWords) > 0 {
		t.re.upperCaseWords = t.newRegex(t.makeCaseInsensitiveWordPattern(cfg.upperCaseWords))
	}

	t.re.lowerCaseWords = neverMatch
	if len(cfg.lowerCaseWords) > 0 {
		t.re.lowerCaseWords = t.newRegex(t.makeCaseInsensitiveWordPattern(cfg.lowerCaseWords))
	}

	t.re.multiPartWords = neverMatch
	if len(cfg.multiPartWords) > 0 {
		t.re.multiPartWords = t.newRegex(t.makeCaseInsensitiveWordPattern(cfg.multiPartWords))
	}

	t.re.ordinalPatterns = neverMatch
	if len(cfg.ordinalPatterns) > 0 {
		t.re.ordinalPatterns = t.newRegex(t.makeCaseInsensitiveWordPattern(cfg.ordinalPatterns))
	}

	// matches strings in computed word lists

	t.re.inseparableWords = neverMatch
	if len(inseparableWords) > 0 {
		words := strings.Join(inseparableWords, "|")
		t.re.inseparableWords = t.newRegex(fmt.Sprintf(`(?i)([^[:alnum:]]*?)(%s)([^[:alnum:]]*)`, words))
	}

	t.re.inseparableParts = neverMatch
	if len(inseparableParts) > 0 {
		parts := strings.Join(inseparableParts, "|")
		t.re.inseparableParts = t.newRegex(fmt.Sprintf(`(?i)^(%s)$`, parts))
	}

	return &t
}

func (t *titleizeContext) capitalize(s string) string {
	if t.re.isCapitalizable.MatchString(s) {
		r := []rune(s)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}

	return s
}

func (t *titleizeContext) toMixedCase(s string) string {
	return t.mixedCaseMap[strings.ToLower(s)]
}

func (t *titleizeContext) toUpperCase(s string) string {
	return strings.ToUpper(s)
}

func (t *titleizeContext) toLowerCase(s string) string {
	return strings.ToLower(s)
}

func (t *titleizeContext) titleize(s string) string {
	t.log("====================================================================================================")

	t.log("old: [%s]", s)

	str := s

	// first, convert word deliminters between inseparable words so that we can treat them as a word "unit" below
	// e.g. if "da vinci" is an inseparable word, then "the da vinci code" becomes "the da}}}!!!{{{vinci code"
	joinedWords := false

	str = t.re.inseparableWords.ReplaceAllStringFunc(str, func(match string) string {
		joinedWords = true

		groups := t.re.inseparableWords.FindStringSubmatch(match)

		// group 2 contains the exactly-matched phrase
		joinedMatch := groups[1] + strings.ReplaceAll(groups[2], " ", t.placeHolder) + groups[3]

		return joinedMatch
	})

	t.log("now: [%s]", str)

	// split out all of the words in the (modified?) input string according to word delimiters
	words := t.re.wordSeparator.Split(str, -1)

	// determine case attributes of this string
	hasNoUpper := (t.re.hasUpperCase.MatchString(str) == false)
	hasNoLower := (t.re.hasLowerCase.MatchString(str) == false)
	isUniform := (hasNoUpper != hasNoLower) && (len(words) > 1)

	t.log("hasNoUpper = %v  hasNoLower = %v  isUniform = %v  len(words) = %d", hasNoUpper, hasNoLower, isUniform, len(words))

	var newWords []string

	isNewPhrase := true

	for _, word := range words {
		// undo any text replacement for inseparable words, e.g. in the example above,
		// the word "da}}}!!!{{{vinci" would become "da vinci" again
		if joinedWords == true {
			word = strings.ReplaceAll(word, t.placeHolder, " ")
		}

		// determine whether to capitalize this word if it begins with a lower case letter,
		// depending on surrounding characters
		capitalizeLowercase := isNewPhrase || hasAnyPrefix(word, []string{"`", "'", `"`})
		isNewPhrase = hasAnySuffix(word, []string{":", ";", "/", "?", "."})

		t.log("this word: [%s]  capitalizeLowercase: %v", word, capitalizeLowercase)

		newWord := word

		if t.re.inseparableParts.MatchString(word) {
			// this word contains a part delimiter that we want to treat as a unit
			// e.g. if "/" is a part delimiter, and "n/a" is a multi-part word
			switch {
			case t.re.mixedCaseWords.MatchString(word):
				newWord = t.toMixedCase(word)
				t.log("inseparable word: [%s] => always mixed => [%s]", word, newWord)

			case t.re.upperCaseWords.MatchString(word):
				newWord = t.toUpperCase(word)
				t.log("inseparable word: [%s] => always upper => [%s]", word, newWord)

			case t.re.lowerCaseWords.MatchString(word):
				newWord = t.toLowerCase(word)
				t.log("inseparable word: [%s] => always lower => [%s]", word, newWord)

			default:
				// this word is not in a case-specific word list, so there is not enough
				// context to decide how to treat it.  just pass it through
				t.log("inseparable word: [%s] => passthrough => [%s]", word, newWord)
			}
		} else {
			// extract the word parts according to the part delimiters, and convert each part
			// according to its content and/or surrounding context
			newWord = t.re.partExtractor.ReplaceAllStringFunc(word, func(part string) string {
				newPart := part

				switch {
				case t.re.mixedCaseWords.MatchString(part):
					newPart = t.toMixedCase(part)
					if capitalizeLowercase == true {
						newPart = t.capitalize(newPart)
					}
					t.log("word: [%s]  part: [%s] => always mixed => [%s]", word, part, newPart)

				case t.re.upperCaseWords.MatchString(part):
					newPart = t.toUpperCase(part)
					t.log("word: [%s]  part: [%s] => always upper => [%s]", word, part, newPart)

				case t.re.areInitials.MatchString(part):
					newPart = t.toUpperCase(part)
					t.log("word: [%s]  part: [%s] => initials => [%s]", word, part, newPart)

				case t.re.lowerCaseWords.MatchString(part):
					if capitalizeLowercase == true {
						newPart = t.capitalize(part)
					} else {
						newPart = t.toLowerCase(part)
					}
					t.log("word: [%s]  part: [%s] => always lower => [%s] (capitalize = %v)", word, part, newPart, capitalizeLowercase)

				case t.re.ordinalPatterns.MatchString(part):
					if isUniform == false {
						newPart = t.toLowerCase(part)
					}
					t.log("word: [%s]  part: [%s] => ordinal => [%s] (isUniform = %v)", word, part, newPart, isUniform)

				case t.re.allUpperCase.MatchString(part):
					// what is this really checking?
					if isUniform == true {
						// i mean, it's all upper case, and so is the entire input string, so this is redundant?
						newPart = t.capitalize(part)
					}
					t.log("word: [%s]  part: [%s] => all upper => [%s] (isUniform = %v)", word, part, newPart, isUniform)

				case t.re.allLowerCase.MatchString(part):
					newPart = t.capitalize(part)
					t.log("word: [%s]  part: [%s] => all lower => [%s]", word, part, newPart)

				default:
					t.log("word: [%s]  part: [%s] => passthrough => [%s]", word, part, newPart)
				}

				// do not capitalize remaining parts in the word
				capitalizeLowercase = false

				return newPart
			})
		}

		newWords = append(newWords, newWord)
	}

	res := strings.Join(newWords, " ")

	t.log("new: [%s]", res)

	return res
}

func (t *titleizeContext) makeCaseInsensitiveWordPattern(words []string) string {
	pattern := fmt.Sprintf("(?i)^([^[:alnum:]]*?)(%s)([^[:alnum:]]*)$", strings.Join(words, "|"))

	return pattern
}

func (t *titleizeContext) newRegex(pattern string) *regexp.Regexp {
	t.log("pattern: %s", pattern)

	return regexp.MustCompile(pattern)
}

func (t *titleizeContext) log(format string, args ...interface{}) {
	if t.debug == false {
		return
	}

	log.Printf("[TITLEIZE] %s", fmt.Sprintf(format, args...))
}
