package updateversioninfile

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
	"os"
	"regexp"
)

const (
	updateVersionInFileCmdStr     = "update-version-in-file"
	versionRegexStr               = "[0-9A-Za-z_./-]+"
	formatStrReplacementSubstr    = "%s"
	expectedNumSearchPatternLines = 1
)

var UpdateVersionInFileCmd = &cobra.Command{
	Use:   updateVersionInFileCmdStr,
	Short: "Updates semantic version of a Kurtosis Repo in file",
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	toUpdateFilepath, patternFormatStr, replacementStr := os.Args[1], os.Args[2], os.Args[3]
	//arg validation:
	//- check args are nonempty
	//- filepath exists
	//- check patternFormatStr contains formatStrReplacementSubstr, "%s"
	//- check that newVersion matches versionRegex pattern

	fileToUpdate, err := os.ReadFile(toUpdateFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read file at '%s'", toUpdateFilepath)
	}
	fileToUpdateInfo, err := os.Stat(toUpdateFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read file at '%s'", toUpdateFilepath)
	}
	fileToUpdateMode := fileToUpdateInfo.Mode()

	searchPatternStr := fmt.Sprintf(patternFormatStr, versionRegexStr)
	searchPatternRegex, err := regexp.Compile(searchPatternStr)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred creating regex pattern of %s", searchPatternStr)
	}

	replaceValue := fmt.Sprintf(patternFormatStr, replacementStr)

	numLines, err := countLinesMatchingRegex(toUpdateFilepath, searchPatternRegex)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to count the number of occurrences of '%s' in '%s'", searchPatternStr, toUpdateFilepath)
	}
	if numLines != expectedNumSearchPatternLines {
		return stacktrace.Propagate(err, "An incorrect amount, '%d' of lines matching '%s' was found in '%s'", num, searchPatternStr, toUpdateFilepath)
	}

	updatedFile := replaceLinesMatchingPatternInFile(replaceValue, searchPatternRegex, fileToUpdate)

	err = os.WriteFile(toUpdateFilepath, updatedFile, fileToUpdateMode)
	if err != nil {
		return stacktrace.Propagate(err, "An error occured attempting to right the updated file contents to '%s'", toUpdateFilepath)
	}
	return nil
}

func replaceLinesMatchingPatternInFile(replacement string, regexPat *regexp.Regexp, file []byte) []byte {
	lines := bytes.Split(file, []byte("\n"))
	for i, line := range lines {
		if regexPat.Match(line) {
			lines[i] = []byte(replacement + "\n")
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func countLinesMatchingRegex(filePath string, regexPat *regexp.Regexp) (int, error) {
	numLinesMatchingPattern := 0
	file, err := os.Open(filePath)
	if err != nil {
		return -1, stacktrace.Propagate(err, "An error occurred while attempting to open file at '%s'", filePath)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if regexPat.Match(scanner.Bytes()) {
			numLinesMatchingPattern++
		}
	}
	if err := scanner.Err(); err != nil {
		return -1, stacktrace.Propagate(err, "An error occurred while scanning file at '%s'", filePath)
	}
	return numLinesMatchingPattern, nil
}
