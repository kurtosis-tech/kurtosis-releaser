package file_line_matcher

import (
	"bufio"
	"github.com/kurtosis-tech/stacktrace"
	"os"
	"regexp"
)

type FileLineMatcher struct{}

func (matcher *FileLineMatcher) MatchNumLines(filePath string, regexPat *regexp.Regexp) (int, error) {
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
