package main

import (
	"bufio"
	"fmt"
	"os"
	"log"
	"path"
	"sort"
	"strings"
	"regexp"
	"time"


	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
)

const (
	gitDirname       = ".git"
	originRemoteName = "origin"
	masterBranchName = "master"
	tagsPrefix       = "refs/tags/"

	relativeChangelogFilepath = "/docs/changelog.md"

	// Taken from guess-release-version.sh
	HEADER_CHAR = "#"
	SEMVER_REGEX = "^[0-9]+.[0-9]+.[0-9]$"
	TBD_VERSION_HEADER_REGEX = "^#[[:space:]]*TBD[[:space:]]*$"
	EXPECTED_NUM_TBD_HEADER_LINES = 1
	VERSION_HEADER_REGEX = "^#[[:space:]]*[0-9]+.[0-9]+.[0-9]+[[:space:]]*$"
	BREAKING_CHANGES_SUBHEADER_REGEX = "^###*[[:space:]]*[Bb]reak.*$"
	NO_PREVIOUS_VERSION = "0.0.0" 
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintln(logrus.StandardLogger().Out, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runMain() error {
	currentWorkingDirectory, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the current working directory.")
	}
	fmt.Printf("Current working directory: %s\n", currentWorkingDirectory)

	gitRepositoryDirectory := path.Join(currentWorkingDirectory, gitDirname)
	if _, err := os.Stat(gitRepositoryDirectory); err != nil {
		if os.IsNotExist(err) {
			return stacktrace.Propagate(err, "An error occurred getting the git repository in this directory. This means that this binary is not being run from root of a git repository.")
		}
	}

	repository, err := git.PlainOpen(currentWorkingDirectory)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to open the existing git repository.")
	}

	_, err = repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	// Check local master branch exists
	localMasterBranch, err := repository.Branch(masterBranchName)
	if err != nil {
		return stacktrace.Propagate(err, "Missing required '%v' branch locally. Please run 'git checkout %v'", masterBranchName, masterBranchName)
	}
	fmt.Printf("Local master branch: %+v\n", localMasterBranch)

	// Check no staged or unstaged changes exist on the branch before release
	worktree, err := repository.Worktree()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the worktree of the repository.")
	}

	currWorktreeStatus, err := worktree.Status()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the status of the worktree of the repository.")
	}
	fmt.Printf("Current working tree status: %s\n", currWorktreeStatus)

	isClean := currWorktreeStatus.IsClean()
	if !isClean {
		fmt.Printf("The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'\n", currWorktreeStatus)
		return nil
	}
	fmt.Printf("Is working tree clean?: %t\n", isClean)

	// Check that local master and remote master are in sync
	localMasterBranchName := masterBranchName
	remoteMasterBranchName := fmt.Sprintf("%v/%v", originRemoteName, masterBranchName)

	localMasterHash, err := repository.ResolveRevision(plumbing.Revision(localMasterBranchName))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", localMasterBranchName)
	}
	remoteMasterHash, err := repository.ResolveRevision(plumbing.Revision(remoteMasterBranchName))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", remoteMasterBranchName)
	}

	fmt.Println(localMasterHash.String())
	fmt.Println(remoteMasterHash.String())
	isLocalMasterInSyncWithRemoteMaster := localMasterHash.String() == remoteMasterHash.String()
	fmt.Printf("Remote Master == Local Master?: %t\n", isLocalMasterInSyncWithRemoteMaster)
	if !isLocalMasterInSyncWithRemoteMaster {
		fmt.Println("The local master branch is not in sync with the remote master branch. Must be in sync to conduct release process.")
		return nil
	}
	
	// Get latest release version
	latestReleaseVersion, err := getLatestReleaseVersion(repository, NO_PREVIOUS_VERSION)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to get the latest release version.")
	}
	fmt.Println(latestReleaseVersion)

	// Look at changelog file to see if it contains `###Breaking Changes` header
	changelogFilepath := path.Join(currentWorkingDirectory, relativeChangelogFilepath)

	// Check that there is the appropriate amount of TBD version headers
	tbdHeaderCount, err := grepFile(changelogFilepath, TBD_VERSION_HEADER_REGEX)
	if tbdHeaderCount != EXPECTED_NUM_TBD_HEADER_LINES {
		fmt.Printf("There should be %d TBD header lines in the changelog. Instead there are %d.\n", EXPECTED_NUM_TBD_HEADER_LINES, tbdHeaderCount)
		return nil
	}

	versionHeaderCount, err := grepFile(changelogFilepath, VERSION_HEADER_REGEX)
	if versionHeaderCount == 0 {
		fmt.Println("No previous changelog versions were detected in this changelog. Are you sure that the changelog is in sync with the release tags on this branch?")
		return nil
	}

	existsBreakingChanges, err := detectBreakingChanges(changelogFilepath, TBD_VERSION_HEADER_REGEX, BREAKING_CHANGES_SUBHEADER_REGEX, VERSION_HEADER_REGEX)
	if err!= nil {
		return stacktrace.Propagate(err, "Error occured while searching for breaking changes.")
	}

	var nextReleaseVersion semver.Version
	if existsBreakingChanges {
		nextReleaseVersion = latestReleaseVersion.IncMinor()
	} else {
		nextReleaseVersion = latestReleaseVersion.IncPatch()
	}

	// Prompt user to see if they want to continue with release with new version
	go wait()
	fmt.Printf("VERIFICATION: Release new version '%s'? (ENTER to continue, Ctrl-C to quit))", nextReleaseVersion.String())
	fmt.Scanln()
	return nil
}

func wait() {
    i := 0
    for {
        time.Sleep(time.Second * 1)
        i++
    }
}

// adapted from: https://stackoverflow.com/questions/26709971/could-this-be-more-efficient-in-go
func grepFile(file string, regexPat string) (int64, error) {
	r, err := regexp.Compile(regexPat)
	if err != nil {
		return 0, stacktrace.Propagate(err, "Could not parse regexp: '%s'", regexPat)
	}
    patCount := int64(0)
    f, err := os.Open(file)
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
		if r.Match(scanner.Bytes()) {
			patCount++
		}
    }
    if err := scanner.Err(); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
    return patCount, nil
}


// Retrieves latest release version from tags in repo
// Returns false
func getLatestReleaseVersion(repo *git.Repository, noPrevVersion string) (semver.Version, error) {
	semverRegex, _ := regexp.Compile(SEMVER_REGEX)
	// should i check for errors here?

	// Grab all tags on the branch
	tagrefs, _ := repo.Tags()
	// should i check for errors here?

	// Trim and filter for only tags with X.Y.Z version format
	var tagSemVers []*semver.Version
	tagrefs.ForEach(func(tagref *plumbing.Reference) error {
		tagName := tagref.Name().String()
		tagName = strings.ReplaceAll(tagName, tagsPrefix, "")
		if !semverRegex.Match([]byte(tagName)) {
			return nil
		}
		tagSemVer, err := semver.StrictNewVersion(tagName)
		if err != nil {
			return err
		}
		// should there be a check here?
		tagSemVers = append(tagSemVers, tagSemVer) 
		return nil
	})
	// should i check for errors here?

	var latestReleaseTagSemVer *semver.Version
	if len(tagSemVers) == 0 {
		latestReleaseTagSemVer, _ = semver.StrictNewVersion(noPrevVersion)
		// should there be a check here?
	} else {
		sort.Sort(sort.Reverse(semver.Collection(tagSemVers)))
		latestReleaseTagSemVer = tagSemVers[0]
	}

	return *latestReleaseTagSemVer, nil
}

// Detects if Breaking Changes Header exists in changelog file based on supplied regex patterns
// If error occurs, returns false, err
// If not, returns true, nil
func detectBreakingChanges(changelogFilepath string, tbdRegexString string, breakingChangesRegexString string, versionHeaderRegexString string) (bool, error) {
	changelogFile, err := os.Open(changelogFilepath);
	if err != nil {
		return false, stacktrace.Propagate(err, "Error attempting to open changelog file at provided path. Are you sure '%s' exists?", changelogFilepath)
	}
	defer changelogFile.Close()

	tbdRegex, err := regexp.Compile(tbdRegexString)
	if err != nil {
		return false, stacktrace.Propagate(err, "Could not parse regexp: '%s'", tbdRegexString)
	}
	breakingChangesRegex, err := regexp.Compile(breakingChangesRegexString)
	if err != nil {
		return false, stacktrace.Propagate(err, "Could not parse regexp: '%s'", breakingChangesRegexString)
	}
	versionHeaderRegex, err := regexp.Compile(versionHeaderRegexString)
	if err != nil {
		return false, stacktrace.Propagate(err, "Could not parse regexp: '%s'", versionHeaderRegexString)
	}

    scanner := bufio.NewScanner(changelogFile)

	// Find TBD header
    for scanner.Scan() {
		if tbdRegex.Match(scanner.Bytes()) {
			break
		}
    }

	// Scan file until next version header detected, detecting breaking changes header along the way
	foundBreakingChanges := false
	for scanner.Scan() {
		if breakingChangesRegex.Match(scanner.Bytes()){
			foundBreakingChanges = true
		}
		
		if versionHeaderRegex.Match(scanner.Bytes()){
			break
		}
	}

    if err := scanner.Err(); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
    return foundBreakingChanges, nil
}