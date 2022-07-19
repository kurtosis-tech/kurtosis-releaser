package getdockertag

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidDockerImgCharsRegex(t *testing.T) {
	validStrings := []string{".1.55", "-1.553", "$docs", "^%$#", "branch/testing"}
	invalidStrings := []string{"1.55.3", "branch-testing", "12340523", "12340523-dirty", "asdf", ""}

	testRegexPattern(t, "Invalid Docker Image Characters Regex", invalidDockerImgCharsRegexStr, validStrings, invalidStrings)
}

// ====================================================================================================
//                                       Private Helper Functions
// ====================================================================================================
func testRegexPattern(t *testing.T, regexPatternName string, regexPatternStr string, validStrings []string, invalidStrings []string) {
	regexPattern := regexp.MustCompile(regexPatternStr)

	for _, str := range validStrings {
		patternDetected := regexPattern.Match([]byte(str))
		require.True(t, patternDetected, "'%s'Pattern was not detected in this string when it should have been: '%s'.", regexPatternName, str)
	}

	for _, str := range invalidStrings {
		patternDetected := regexPattern.Match([]byte(str))
		require.False(t, patternDetected, "'%s' Pattern was detected in this string when it should not have been: '%s'.", regexPatternName, str)
	}
}
