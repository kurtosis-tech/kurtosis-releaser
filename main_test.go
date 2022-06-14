package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain_TBDHeaderRegex(t *testing.T){
	tbdHeaderRegexStr := "^#[[:space:]]*TBD[[:space:]]*$"
	tbdHeaderRegex := regexp.MustCompile(tbdHeaderRegexStr)

	validTBDHeaderStr := "# TBD"
	tbdHeaderDetected := tbdHeaderRegex.Match([]byte(validTBDHeaderStr))

	require.True(t, tbdHeaderDetected, "TBD Header was not detected in this string when it should have been: '%s'.", validTBDHeaderStr)
}

func TestMain_VersionHeaderRegex(t *testing.T){
	versionHeaderRegexStr := "^#[[:space:]]*[0-9]+.[0-9]+.[0-9]+[:space:]]*$"
	versionHeaderRegex := regexp.MustCompile(versionHeaderRegexStr)

	validVersionHeaderStr := "# 1.54.2"
	versionHeaderDetected := versionHeaderRegex.Match([]byte(validVersionHeaderStr))

	require.True(t, versionHeaderDetected, "Version header was not detected in this string when it should have been: '%s'.", validVersionHeaderStr)
}

func TestMain_BreakingChangesRegex(t *testing.T){
	breakingChangesSubheaderRegexStr := "^###*[[:space:]]*[Bb]reak.*$"
	breakingChangesRegex := regexp.MustCompile(breakingChangesSubheaderRegexStr)

	validBreakingChangesStr := "### Breaking Changes"
	breakingChangesDetected := breakingChangesRegex.Match([]byte(validBreakingChangesStr))

	require.True(t, breakingChangesDetected, "Breaking changes was not detected in this string when it should have been: '%s'.", validBreakingChangesStr)
}