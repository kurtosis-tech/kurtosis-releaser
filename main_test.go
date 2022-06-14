package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain_SemverRegex(t *testing.T){
	semverRegexStr := "^[0-9]+.[0-9]+.[0-9]$"
	semverRegex := regexp.MustCompile(semverRegexStr)

	validSemverStr := "0.0.0"
	semverDetected := semverRegex.Match([]byte(validSemverStr))

	require.True(t, semverDetected, "Semver Header was not detected in this string when it should have been: '%s'.", validSemverStr)
}

func TestMain_TBDHeaderRegex(t *testing.T){
	tbdHeaderRegexStr := "^#\\s*TBD\\s*$"
	tbdHeaderRegex := regexp.MustCompile(tbdHeaderRegexStr)

	validTBDHeaderStr := "# TBD"
	tbdHeaderDetected := tbdHeaderRegex.Match([]byte(validTBDHeaderStr))

	require.True(t, tbdHeaderDetected, "TBD Header was not detected in this string when it should have been: '%s'.", validTBDHeaderStr)
}

func TestMain_VersionHeaderRegex(t *testing.T){
	versionHeaderRegexStr := "^#\\s*[0-9]+.[0-9]+.[0-9]+\\s*$"
	versionHeaderRegex := regexp.MustCompile(versionHeaderRegexStr)

	validVersionHeaderStr := "# 1.54.2"
	versionHeaderDetected := versionHeaderRegex.Match([]byte(validVersionHeaderStr))

	require.True(t, versionHeaderDetected, "Version header was not detected in this string when it should have been: '%s'.", validVersionHeaderStr)
}

func TestMain_BreakingChangesRegex(t *testing.T){
	breakingChangesSubheaderRegexStr := "^###*\\s*[Bb]reak.*$"
	breakingChangesRegex := regexp.MustCompile(breakingChangesSubheaderRegexStr)

	validBreakingChangesStr := "### Breaking Changes"
	breakingChangesDetected := breakingChangesRegex.Match([]byte(validBreakingChangesStr))

	require.True(t, breakingChangesDetected, "Breaking changes was not detected in this string when it should have been: '%s'.", validBreakingChangesStr)
}