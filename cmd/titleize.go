package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode"
)

// titleization logic

type titleizeREs struct {
	wordSeparators    *regexp.Regexp
	partExtractor     *regexp.Regexp
	mixedCaseWords    *regexp.Regexp
	upperCaseWords    *regexp.Regexp
	lowerCaseWords    *regexp.Regexp
	ordinalPatterns   *regexp.Regexp
	treatAsUnit       *regexp.Regexp
	spaceWords        *regexp.Regexp
	spaceWordsMatcher *regexp.Regexp
	initials          *regexp.Regexp
	hasUpperCase      *regexp.Regexp
	hasLowerCase      *regexp.Regexp
	allUpperCase      *regexp.Regexp
	allLowerCase      *regexp.Regexp
	isCapitalizable   *regexp.Regexp
}

type titleizeContext struct {
	re           titleizeREs
	placeHolder  string
	mixedCaseMap map[string]string
	debug        bool
}

func newTitleizeContext() *titleizeContext {
	t := titleizeContext{
		placeHolder: `}}}!!!{{{`,
	}

	mixedCaseWords := []string{
		`da Vinci`,
		`Los Angeles`,
		`Va\.`,
		`Va`,
	}

	upperCaseWords := []string{
		`TAHPERD`,
		`ICASE`,
		`ICPSR`,
		`JSTOR`,
		`NIOSH`,
		`USITC`,
		`XVIII`,
		`AHRQ`,
		`DHEW`,
		`DHHS`,
		`IEEE`,
		`ISBN`,
		`ISSN`,
		`NASA`,
		`NATO`,
		`NCAA`,
		`NIMH`,
		`NTIA`,
		`PLOS`,
		`SPIE`,
		`USAF`,
		`USMC`,
		`VIII`,
		`VTRC`,
		`XIII`,
		`XVII`,
		`A & E`,
		`ABA`,
		`ABC`,
		`AEI`,
		`ASI`,
		`BMJ`,
		`CBO`,
		`CLE`,
		`CMH`,
		`CNN`,
		`DNA`,
		`HHS`,
		`IBM`,
		`III`,
		`MIT`,
		`NAA`,
		`NIH`,
		`OCS`,
		`PBS`,
		`R&D`,
		`SSA`,
		`USA`,
		`USN`,
		`VII`,
		`WTO`,
		`XII`,
		`XIV`,
		`XIX`,
		`XVI`,
		`DA`,
		`II`,
		`IV`,
		`IX`,
		`UK`,
		`XI`,
		`XV`,
		`XX`,
		`I`,
	}

	lowerCaseWords := []string{
		`van der`,
		`van den`,
		`and`,
		`ca\.`,
		`del`,
		`des`,
		`etc`,
		`for`,
		`les`,
		`los`,
		`nor`,
		`pp\.`,
		`the`,
		`und`,
		`van`,
		`von`,
		`vs\.`,
		`an`,
		`as`,
		`at`,
		`by`,
		`de`,
		`di`,
		`du`,
		`el`,
		`et`,
		`in`,
		`la`,
		`le`,
		`of`,
		`on`,
		`or`,
		`to`,
		`v\.`,
		`a`,
		`à`,
		`e`,
		`o`,
		`y`,
	}

	ordinalPatterns := []string{
		`1st`,
		`2nd`,
		`3rd`,
		`[04-9]th`,
		`\d*1\dth`,
		`\d*[2-9]1st`,
		`\d*[2-9]2nd`,
		`\d*[2-9]3rd`,
		`\d*[2-9][4-9]th`,
	}

	// phrases containing parts separator(s) that should be treated as a unit
	treatAsUnit := []string{
		`n/a`,
	}

	t.mixedCaseMap = make(map[string]string)
	for _, word := range mixedCaseWords {
		t.mixedCaseMap[strings.ToLower(word)] = word
	}

	var allWords []string
	allWords = append(allWords, mixedCaseWords...)
	allWords = append(allWords, upperCaseWords...)
	allWords = append(allWords, lowerCaseWords...)
	allWords = append(allWords, ordinalPatterns...)

	var spaceWords []string

	for _, word := range allWords {
		if strings.Contains(word, " ") {
			spaceWords = append(spaceWords, word)
		}
	}

	t.re.wordSeparators = t.newRegex(`[[:space:]]+`)
	t.re.partExtractor = t.newRegex(`[^:;/+-]+`)
	t.re.hasUpperCase = t.newRegex(`.*[[:upper:]].*`)
	t.re.hasLowerCase = t.newRegex(`.*[[:lower:]].*`)
	t.re.isCapitalizable = t.newRegex(`^[[:lower:]]($|[[:lower:]])`)
	//t.re.IsCapitalizable = t.newRegex(`^[[:lower:]][[:lower:][:space:]]`)

	t.re.initials = t.newRegex(t.makePattern([]string{`(?:[[:alpha:]]\.)+`}))
	t.re.allUpperCase = t.newRegex(`^[[:upper:]][^[:lower:]]*$`)
	t.re.allLowerCase = t.newRegex(`^[[:lower:]][^[:upper:]]*$`)

	t.re.mixedCaseWords = t.newRegex(t.makePattern(mixedCaseWords))
	t.re.upperCaseWords = t.newRegex(t.makePattern(upperCaseWords))
	t.re.lowerCaseWords = t.newRegex(t.makePattern(lowerCaseWords))
	t.re.ordinalPatterns = t.newRegex(t.makePattern(ordinalPatterns))
	t.re.treatAsUnit = t.newRegex(t.makePattern(treatAsUnit))

	if len(spaceWords) > 0 {
		pattern := fmt.Sprintf(`(?i)(%s)`, strings.Join(spaceWords, "|"))
		t.re.spaceWords = t.newRegex(pattern)

		pattern = fmt.Sprintf(`(?i)([^[:alnum:]]*)(%s)([^[:alnum:]]*)`, strings.Join(spaceWords, "|"))
		t.re.spaceWordsMatcher = t.newRegex(pattern)
	}

	return &t
}

func (t *titleizeContext) capitalize(b []byte) []byte {
	if t.re.isCapitalizable.Match(b) {
		r := []rune(string(b))
		r[0] = unicode.ToUpper(r[0])
		return []byte(string(r))
	}

	return b
}

func (t *titleizeContext) mixedcase(b []byte) []byte {
	s := string(b)
	return []byte(t.mixedCaseMap[strings.ToLower(s)])
}

func (t *titleizeContext) upcase(b []byte) []byte {
	s := string(b)
	return []byte(strings.ToUpper(s))
}

func (t *titleizeContext) downcase(b []byte) []byte {
	s := string(b)
	return []byte(strings.ToLower(s))
}

func (t *titleizeContext) titleize(s string) string {
	t.log("====================================================================================================")

	t.log("[TITLEIZE] old: [%s]", s)

	str := s

	placeHolder := ""

	if t.re.spaceWords != nil {
		placeHolder = t.placeHolder

		newBytes := t.re.spaceWordsMatcher.ReplaceAllFunc([]byte(str), func(match []byte) []byte {
			newMatch := t.re.spaceWords.ReplaceAllFunc([]byte(match), func(phrase []byte) []byte {
				newPhrase := strings.ReplaceAll(string(phrase), " ", placeHolder)
				//t.log("[TITLEIZE] space phrase: [%s] => [%s]", phrase, newPhrase)
				return []byte(newPhrase)
			})

			//t.log("[TITLEIZE] space match: [%s] => [%s]", string(match), string(newMatch))
			return newMatch
		})

		str = string(newBytes)
	}

	words := t.re.wordSeparators.Split(str, -1)

	noUpper := (t.re.hasUpperCase.MatchString(str) == false)
	noLower := (t.re.hasLowerCase.MatchString(str) == false)
	uniform := (noUpper != noLower) && (len(words) > 1)

	var newWords []string

	newPhrase := true

	for _, word := range words {
		if placeHolder != "" {
			word = strings.ReplaceAll(word, placeHolder, " ")
		}

		capitalizeLowerCase := newPhrase || hasAnyPrefix(word, []string{"`", "'", `"`})
		newPhrase = hasAnySuffix(word, []string{":", ";", "/", "?", "."})

		//t.log("[TITLEIZE] word: [%s]  capitalizeLowerCase: %v", word, capitalizeLowerCase)

		var newBytes []byte

		if t.re.treatAsUnit.MatchString(word) {
			switch {
			case t.re.mixedCaseWords.MatchString(word):
				newBytes = t.mixedcase([]byte(word))
				t.log("[TITLEIZE] word: [%s] => always mixed => [%s]", word, string(newBytes))

			case t.re.upperCaseWords.MatchString(word):
				t.log("[TITLEIZE] word: [%s] => always upper => [%s]", word, string(newBytes))
				newBytes = t.upcase([]byte(word))

			case t.re.lowerCaseWords.MatchString(word):
				t.log("[TITLEIZE] word: [%s] => always lower => [%s]", word, string(newBytes))
				newBytes = t.downcase([]byte(word))

			default:
				if capitalizeLowerCase == true {
					newBytes = t.upcase([]byte(word))
				} else {
					newBytes = t.downcase([]byte(word))
				}
				t.log("[TITLEIZE] word: [%s] => other => [%s] (capitalize = %v)", word, string(newBytes), capitalizeLowerCase)
			}
		} else {
			newBytes = t.re.partExtractor.ReplaceAllFunc([]byte(word), func(part []byte) []byte {
				res := part

				switch {
				case t.re.mixedCaseWords.Match(part):
					res = t.mixedcase(part)
					if capitalizeLowerCase == true {
						res = t.capitalize(res)
					}
					t.log("[TITLEIZE] word: [%s]  part: [%s] => always mixed => [%s]", word, string(part), string(res))

				case t.re.upperCaseWords.Match(part):
					res = t.upcase(part)
					t.log("[TITLEIZE] word: [%s]  part: [%s] => always upper => [%s]", word, string(part), string(res))

				case t.re.initials.Match(part):
					res = t.upcase(part)
					t.log("[TITLEIZE] word: [%s]  part: [%s] => initials => [%s]", word, string(part), string(res))

				case t.re.lowerCaseWords.Match(part):
					if capitalizeLowerCase == true {
						res = t.capitalize(part)
					} else {
						res = t.downcase(part)
					}
					t.log("[TITLEIZE] word: [%s]  part: [%s] => always lower => [%s] (capitalize = %v)", word, string(part), string(res), capitalizeLowerCase)

				case t.re.ordinalPatterns.Match(part):
					if uniform == false {
						res = t.downcase(part)
					}
					t.log("[TITLEIZE] word: [%s]  part: [%s] => ordinal => [%s] (uniform = %v)", word, string(part), string(res), uniform)

				case t.re.allUpperCase.Match(part):
					// what is this really checking?
					if uniform == true {
						res = t.capitalize(part)
					}
					t.log("[TITLEIZE] word: [%s]  part: [%s] => all upper => [%s] (uniform = %v)", word, string(part), string(res), uniform)

				case t.re.allLowerCase.Match(part):
					res = t.capitalize(part)
					t.log("[TITLEIZE] word: [%s]  part: [%s] => all lower => [%s]", word, string(part), string(res))

				default:
					t.log("[TITLEIZE] word: [%s]  part: [%s] => passthrough => [%s]", word, string(part), string(res))
				}

				capitalizeLowerCase = false

				return res
			})
		}

		newWords = append(newWords, string(newBytes))
	}

	res := strings.Join(newWords, " ")

	t.log("[TITLEIZE] new: [%s]", res)

	return res
}

func (t *titleizeContext) makePattern(words []string) string {
	pattern := fmt.Sprintf("(?i)^([^[:alnum:]]*?)(%s)([^[:alnum:]]*)$", strings.Join(words, "|"))

	return pattern
}

func (t *titleizeContext) newRegex(pattern string) *regexp.Regexp {
	//t.log("pattern: [%s]", pattern)

	return regexp.MustCompile(pattern)
}

func (t *titleizeContext) log(format string, args ...interface{}) {
	if t.debug == false {
		return
	}

	log.Printf("[TITLEIZE] %s", fmt.Sprintf(format, args...))
}
