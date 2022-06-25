package updateversioninfile

import (
	"bytes"
	"fmt"
	"github.com/kurtosis-tech/kudet/helpers"
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
	Short: "Updates semantic version of a Kurtosis Repo in file",
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
	fileToUpdateMode := fileToUpdateInfo.Mode()
	fileToUpdate, err := os.ReadFile(toUpdateFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read file at '%s'", toUpdateFilepath)
	}

	if !strings.Contains(patternFormatStr, formatStrReplacementSubstr) {
		return stacktrace.NewError("The replacement substring '%s' was not found in the passed '%s' as required.", formatStrReplacementSubstr, patternFormatStr)
	}

	if !versionRegex.Match([]byte(newVersion)) {
		return stacktrace.NewError("The version regex pattern, '%s', was not found in the provided version, '%s'", versionRegexStr, newVersion)
	}

	searchPatternStr := fmt.Sprintf(patternFormatStr, versionRegexStr)
	searchPatternRegex, err := regexp.Compile(searchPatternStr)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred creating regex pattern of %s", searchPatternStr)
	}

	replaceValue := fmt.Sprintf(patternFormatStr, newVersion)

	numLines, err := helpers.CountLinesMatchingRegex(toUpdateFilepath, searchPatternRegex)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to count the number of occurrences of '%s' in '%s'", searchPatternStr, toUpdateFilepath)
	}
	if numLines != expectedNumSearchPatternLines {
		return stacktrace.NewError("An incorrect amount, '%d' of lines matching '%s' was found in '%s'. '%d' matching lines were expected.", numLines, searchPatternStr, toUpdateFilepath, expectedNumSearchPatternLines)
	}

	updatedFile := replaceLinesMatchingPattern(fileToUpdate, searchPatternRegex, replaceValue)

	err = os.WriteFile(toUpdateFilepath, updatedFile, fileToUpdateMode)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to right the updated file contents to '%s'", toUpdateFilepath)
	}
	return nil
}

func replaceLinesMatchingPattern(file []byte, regexPat *regexp.Regexp, replacement string) []byte {
	lines := bytes.Split(file, []byte("\n"))
	for i, line := range lines {
		if regexPat.Match(line) {
			lines[i] = []byte(replacement)
		}
	}
	return bytes.Join(lines, []byte("\n"))
}
