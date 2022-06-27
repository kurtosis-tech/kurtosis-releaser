package updateversioninfile

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func TestNoMatchingPatternFoundReturnsIdenticalFile(t *testing.T) {
	replacementStr := "KURTOSIS_CORE_VERSION: string = \"0.1.3\""
	searchPatternStr := fmt.Sprintf("KURTOSIS_CORE_VERSION: string = \"%s\"", versionRegexStr)
	searchPatternRegex := regexp.MustCompile(searchPatternStr)

	fileWithNoMatchingPattern :=
		`// DO NOT UPDATE, MANUALLY UPDATED

### A Big change
* Something`

	updatedFile := replaceLinesMatchingPattern([]byte(fileWithNoMatchingPattern), searchPatternRegex, replacementStr)

	require.Equal(t, string(updatedFile), string(fileWithNoMatchingPattern))
}

func TestMatchingPatternFoundReturnsUpdatedLine(t *testing.T) {
	replacementStr := "KURTOSIS_CORE_VERSION: string = \"0.1.3\""
	searchPatternStr := fmt.Sprintf("KURTOSIS_CORE_VERSION: string = \"%s\"", versionRegexStr)
	searchPatternRegex := regexp.MustCompile(searchPatternStr)

	fileWithMatchingPattern :=
		`// DO NOT UPDATE, MANUALLY UPDATED
KURTOSIS_CORE_VERSION: string = "1.5.2"
### A Big change
* Something Else`

	updatedFileWithReplacedLine :=
		`// DO NOT UPDATE, MANUALLY UPDATED
KURTOSIS_CORE_VERSION: string = "0.1.3"
### A Big change
* Something Else`

	updatedFile := replaceLinesMatchingPattern([]byte(fileWithMatchingPattern), searchPatternRegex, replacementStr)

	require.Equal(t, string(updatedFile), string(updatedFileWithReplacedLine))
}

func TestMultipleMatchingPatternsFoundReturnsUpdatesLines(t *testing.T) {
	replacementStr := "KURTOSIS_CORE_VERSION: string = \"0.1.3\""
	searchPatternStr := fmt.Sprintf("KURTOSIS_CORE_VERSION: string = \"%s\"", versionRegexStr)
	searchPatternRegex := regexp.MustCompile(searchPatternStr)

	fileWithMultipleLinesMatchingPattern :=
		`// DO NOT UPDATE, MANUALLY UPDATED
KURTOSIS_CORE_VERSION: string = "1.5.2"
### A Big change
* Something Else
KURTOSIS_CORE_VERSION: string = "1.5.2"`

	updatedFileWithReplacedLines :=
		`// DO NOT UPDATE, MANUALLY UPDATED
KURTOSIS_CORE_VERSION: string = "0.1.3"
### A Big change
* Something Else
KURTOSIS_CORE_VERSION: string = "0.1.3"`

	updatedFile := replaceLinesMatchingPattern([]byte(fileWithMultipleLinesMatchingPattern), searchPatternRegex, replacementStr)

	require.Equal(t, string(updatedFile), string(updatedFileWithReplacedLines))
}

func TestVersionRegexPattern(t *testing.T) {
	validStrings := []string{"1.2.3", "tedisVersion", "10234-dirty", "1-2-3", "%thisTypeOfVersion"}
	invalidStrings := []string{"#%^", " ", "", ""}

	testRegexPattern(t, "Version Regex", versionRegexStr, validStrings, invalidStrings)
}

func testRegexPattern(t *testing.T, regexPatternName string, regexPatternStr string, validStrings []string, invalidStrings []string) {
	regexPattern := regexp.MustCompile(regexPatternStr)

	for _, str := range validStrings {
		patternDetected := regexPattern.Match([]byte(str))
		require.True(t, patternDetected, "%s Pattern was not detected in this string when it should have been: '%s'.", regexPatternName, str)
	}

	for _, str := range invalidStrings {
		patternDetected := regexPattern.Match([]byte(str))
		require.False(t, patternDetected, "%s Pattern was detected in this string when it should not have been: '%s'.", regexPatternName, str)
	}
}
