package release

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemverRegex(t *testing.T) {
	validStrings := []string{"0.0.0", "1.26.11234", "0.1.11", "1.2.3"}
	invalidStrings := []string{" 0.0.0", "1.1", ".5.6", "1.2.", "..", "0.0.0 "}

	testRegexPattern(t, "Semver", semverRegexStr, validStrings, invalidStrings)
}

func TestVersionToBeReplacedPlaceholderHeaderRegex(t *testing.T) {
	validStrings := []string{"# TBD", "# TBD  ", "#TBD"}
	invalidStrings := []string{"## TBD", "# TD "}

	testRegexPattern(t, "Version to Be Replaced Placeholder Header", versionToBeReleasedPlaceholderHeaderRegexStr, validStrings, invalidStrings)
}

func TestVersionHeaderRegex(t *testing.T) {
	validStrings := []string{"# 1.54.2", "#1.5.2"}
	invalidStrings := []string{"## 1.54.2", "1.5.2", "# ..", "# 1.52.", "# 1..25", "# 1.52"}

	testRegexPattern(t, "Version Header", versionHeaderRegexStr, validStrings, invalidStrings)
}

func TestBreakingChangesSubheaderRegex(t *testing.T) {
	validStrings := []string{"### Breaking Changes", "### breaking changes", "### break", "## Breaking Chages", "###BreakingChanges", "### Break"}
	invalidStrings := []string{"Breaking Changes", "### Breking Changes", " ## Break"}

	testRegexPattern(t, "Breaking Changes Subheader", breakingChangesSubheaderRegexStr, validStrings, invalidStrings)
}

func Test_parseChangeLogFileNegativeTest(t *testing.T) {

	// test inputs
	noVersionFound :=
		`#TBD
* Something
* Something else`

	tbdNotPresent :=
		`
* Something
* Something else`

	multipleTBDFound :=
		`# TBD
* Something
# TBD
* Something else`

	noNewUpdatesForCurrentRelease :=
		`# TBD

		
# 0.1.0
* Something else`

	type args struct {
		changelogFile string
	}

	tests := []struct {
		name     string
		args     args
		want     bool
		wantErr  bool
		errorMsg string
	}{
		{
			name: "noVersionFound",
			args: args{
				changelogFile: noVersionFound,
			},
			wantErr:  true,
			want:     false,
			errorMsg: "No previous release versions were detected in this changelog",
		},
		{
			name: "tbdNotPresent",
			args: args{
				changelogFile: tbdNotPresent,
			},
			wantErr:  true,
			want:     false,
			errorMsg: "TBD header not found while reading changelog.md",
		},
		{
			name: "multipleTBDFound",
			args: args{
				changelogFile: multipleTBDFound,
			},
			wantErr:  true,
			want:     false,
			errorMsg: fmt.Sprintf("Found more than %d TBD headers", expectedNumTBDHeaderLines),
		},
		{
			name: "noNewUpdatesForCurrentRelease",
			args: args{
				changelogFile: noNewUpdatesForCurrentRelease,
			},
			wantErr:  true,
			want:     false,
			errorMsg: "changelog.md is empty for the current release",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseChangeLogFile([]byte(tt.args.changelogFile))
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseChangeLogFileNegativeTest() wanted error but received null")
				}
				require.ErrorContains(t, err, tt.errorMsg, "parseChangeLogFileNegativeTest() should throw error")
				return
			}
		})
	}
}

func TestDoBreakingChangesExistIfChangelogIsValid(t *testing.T) {
	onlyOneVersion :=
		`#TBD
* Something

#0.1.0
* Something`

	onlyOneVersionWithSpaces :=
		`# TBD
* Something

# 0.1.0
* Something`

	onlyOneVersionTwoHashBreakingChanges :=
		`#TBD
* Something

##Breaking Changes
* Something else

#0.1.0
* Something`

	onlyOneVersionThreeHashBreakingChanges :=
		`#TBD
* Something

###Breaking Changes
* Something else

#0.1.0
* Something`

	onlyOneVersionFourHashBreakingChanges :=
		`#TBD
* Something

####Breaking Changes
* Something else

#0.1.0
* Something`

	multipleVersions :=
		`#TBD
* Something

#0.1.1
* Something else

#0.1.0
* Something`

	multipleVersionsBreakingChanges :=
		`#TBD
* Something

### Breaking Changes
* Something

#0.1.1
* Something else

#0.1.0
* Something`

	lowercaseBreakingChanges :=
		`# TBD
### breaking changes
* Some breaks

# 0.1.0
* Something`

	shouldHaveBreakingChanges := []string{onlyOneVersionTwoHashBreakingChanges, onlyOneVersionThreeHashBreakingChanges, onlyOneVersionFourHashBreakingChanges, multipleVersionsBreakingChanges, lowercaseBreakingChanges}
	shouldNotHaveBreakingChanges := []string{onlyOneVersion, onlyOneVersionWithSpaces, multipleVersions}
	testBreakingChangesExists(t, shouldHaveBreakingChanges, shouldNotHaveBreakingChanges)
}

func TestIsWhiteSpaceOrPattern_IdentifiesComment(t *testing.T) {
	testCase := "# this is a comment"
	require.True(t, isWhiteSpaceOrComment(testCase))
}

func TestIsWhiteSpaceOrPattern_IdentifiesPureWhiteSpaceAndNewLines(t *testing.T) {
	testCases := []string{
		" ",
		"    ",
		"\n  ",
	}
	for _, testCase := range testCases {
		require.True(t, isWhiteSpaceOrComment(testCase))
	}
}

func TestIsWhiteSpaceOrPattern_IdentifiesActuallyUsefulIgnores(t *testing.T) {
	testCases := []string{
		"kurtosis_version/kurtosis_version.go",
		" long file with spaces around it ",
		"*.pyc",
	}
	for _, testCase := range testCases {
		require.False(t, isWhiteSpaceOrComment(testCase))
	}
}

// ====================================================================================================
//
//	Private Helper Functions
//
// ====================================================================================================
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

func testBreakingChangesExists(t *testing.T, validStrings []string, invalidStrings []string) {
	for _, str := range validStrings {
		hasBreakingChanges, err := parseChangeLogFile([]byte(str))
		require.NoError(t, err, "An error occurred testing if breaking changes existed.")
		require.True(t, hasBreakingChanges, "Breaking Changes were not detected in this string when it should have been:\n%s", str)
	}

	for _, str := range invalidStrings {
		hasBreakingChanges, err := parseChangeLogFile([]byte(str))
		require.NoError(t, err, "An error occurred testing if breaking changes existed.")
		require.False(t, hasBreakingChanges, "Breaking Changes were detected in this string when it should not have been:\n%s", str)
	}
}
