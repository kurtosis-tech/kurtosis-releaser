package main

import (
	"regexp"
	"testing"
	"fmt"

	"github.com/stretchr/testify/require"
)

const (
	sectionHeaderPrefix = "#"
	versionToBeReleasedPlaceholderStr = "TBD"
)

func TestMain_SemverRegex(t *testing.T){
	semverRegexStr := "^[0-9]+.[0-9]+.[0-9]+$"
	
	validStrings := []string{"0.0.0", "1.26.11234", "0.1.11", "1.2.3"}
	invalidStrings := []string{" 0.0.0", "1.1", ".5.6", "1.2.", "..", "0.0.0 "}

	testRegexPattern(t, "Semver", semverRegexStr, validStrings, invalidStrings)
}

func TestMain_TBDHeaderRegex(t *testing.T){
	tbdHeaderRegexStr := fmt.Sprintf("^%s\\s*%s\\s*$", sectionHeaderPrefix, versionToBeReleasedPlaceholderStr)
	
	validStrings := []string{"# TBD", "# TBD  ", "#TBD"}
	invalidStrings := []string{"## TBD", "# TD "}

	testRegexPattern(t, "TBD Header", tbdHeaderRegexStr, validStrings, invalidStrings)
}

func TestMain_VersionHeaderRegex(t *testing.T){
	versionHeaderRegexStr := fmt.Sprintf("^%s\\s*[0-9]+.[0-9]+.[0-9]+\\s*$", sectionHeaderPrefix)

	validStrings := []string{"# 1.54.2", "#1.5.2"}
	invalidStrings := []string{"## 1.54.2", "1.5.2", "# ..", "# 1.52.", "# 1..25", "# 1.52"}

	testRegexPattern(t, "Version Header", versionHeaderRegexStr, validStrings, invalidStrings)
}

func TestMain_BreakingChangesSubheaderRegex(t *testing.T){
	breakingChangesSubheaderRegexStr := fmt.Sprintf("^%s%s%s*\\s*[Bb]reak.*$", sectionHeaderPrefix, sectionHeaderPrefix, sectionHeaderPrefix)

	validStrings := []string{"### Breaking Changes", "### breaking changes", "### break", "## Breaking Chages", "###BreakingChanges", "### Break"}
	invalidStrings := []string{"Breaking Changes", "### Breking Changes", " ## Break"}

	testRegexPattern(t, "Breaking Changes Subheader", breakingChangesSubheaderRegexStr, validStrings, invalidStrings)
}

func testRegexPattern(t *testing.T, regexPatternName string, regexPatternStr string, validStrings []string, invalidStrings []string) {
	regexPattern := regexp.MustCompile(regexPatternStr)

	for _, str := range(validStrings)  {
		patternDetected := regexPattern.Match([]byte(str))
		require.True(t, patternDetected, "%s Pattern was not detected in this string when it should have been: '%s'.", regexPatternName, str)
	}

	for _, str := range(invalidStrings)  {
		patternDetected := regexPattern.Match([]byte(str))
		require.False(t, patternDetected, "%s Pattern was detected in this string when it should not have been: '%s'.", regexPatternName, str)
	}
}