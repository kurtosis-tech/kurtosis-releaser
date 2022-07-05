package updateversioninfile

import (
	"fmt"
	"github.com/kurtosis-tech/kudet/commands_shared_code/file_line_matcher"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
	"os"
	"regexp"
	"strings"
)

const (
	updateVersionInFileCmdStr     = "update-version-in-file <to update filepath> <pattern format string> <new version>"
	versionRegexStr               = "[0-9A-Za-z_./-]+"
	formatStrReplacementSubstr    = "%s"
	expectedNumSearchPatternLines = 1
)

var versionRegex = regexp.MustCompile(versionRegexStr)

var UpdateVersionInFileCmd = &cobra.Command{
	Use:   updateVersionInFileCmdStr,
	Short: "Updates version line",
	Long:  "Updates line in a file containing a Kurtosis repo version to a new version",
	Args:  cobra.ExactArgs(3),
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	toUpdateFilepath, patternFormatStr, newVersion := args[0], args[1], args[2]
	fileToUpdateInfo, err := os.Stat(toUpdateFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			return stacktrace.Propagate(err, "No file exists at '%s'", toUpdateFilepath)
		}
		return stacktrace.Propagate(err, "An error occurred attempting to retrieve file info for file at '%s'", toUpdateFilepath)
	}
	if !strings.Contains(patternFormatStr, formatStrReplacementSubstr) {
		return stacktrace.NewError("The replacement substring '%s' was not found in the provided match regex '%s' as required.", formatStrReplacementSubstr, patternFormatStr)
	}
	if !versionRegex.Match([]byte(newVersion)) {
		return stacktrace.NewError("The provided version '%s' does not match the version regex '%s'", newVersion, versionRegexStr)
	}

	fileToUpdateMode := fileToUpdateInfo.Mode()
	fileToUpdateBytes, err := os.ReadFile(toUpdateFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read file at '%s'", toUpdateFilepath)
	}

	searchPatternStr := fmt.Sprintf(patternFormatStr, versionRegexStr)
	searchPatternRegex, err := regexp.Compile(searchPatternStr)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred creating regex pattern of '%s'", searchPatternStr)
	}

	replaceValue := fmt.Sprintf(patternFormatStr, newVersion)

	matcher := file_line_matcher.FileLineMatcher{}
	numLines, err := matcher.MatchNumLines(toUpdateFilepath, searchPatternRegex)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to count the number of occurrences of '%s' in '%s'", searchPatternStr, toUpdateFilepath)
	}
	if numLines != expectedNumSearchPatternLines {
		return stacktrace.NewError("An incorrect amount, '%d' of lines matching '%s' was found in '%s'. '%d' matching lines were expected.", numLines, searchPatternStr, toUpdateFilepath, expectedNumSearchPatternLines)
	}

	// TODO This reads a file of arbitrary size into memory, file should be updated via streaming via Scanner instead
	updatedFileBytes := replaceLinesMatchingPattern(fileToUpdateBytes, searchPatternRegex, replaceValue)

	err = os.WriteFile(toUpdateFilepath, updatedFileBytes, fileToUpdateMode)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to right the updated file contents to '%s'", toUpdateFilepath)
	}
	return nil
}

func replaceLinesMatchingPattern(file []byte, regexPat *regexp.Regexp, replacement string) []byte {
	return regexPat.ReplaceAll(file, []byte(replacement))
}
