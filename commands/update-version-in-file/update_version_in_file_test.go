package updateversioninfile

import (
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func TestNoMatchingPatternFoundReturnsIdenticalFile(t *testing.T) {
	replacementStr := "KURTOSIS_CORE_VERSION: string = \"0.1.3\""
	searchPatternStr := "KURTOSIS_CORE_VERSION: string = \"[0-9A-Za-z_./-]+\""
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
	searchPatternStr := "KURTOSIS_CORE_VERSION: string = \"[0-9A-Za-z_./-]+\""
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
	searchPatternStr := "KURTOSIS_CORE_VERSION: string = \"[0-9A-Za-z_./-]+\""
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
