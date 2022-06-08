package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"regexp"
	"time"


	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
)

const (
	gitDirname = ".git"
	gitUsername = "git"
	originRemoteName = "origin"
	masterBranchName = "master"
	emptyPassword = ""
	tagsPrefix = "refs/tags/"

	// The name of the file inside the Git directory which will store when we last fetched (in Unix seconds)
	lastFetchedFilename = "last-fetch.txt"

	lastFetchedTimestampUintParseBase = 10
	lastFetchedTimestampUintParseBits = 64
	
	// How long we'll allow the user to go between fetches to ensure the repo is updated when they're releasing
	fetchGracePeriod = 30 * time.Second
	extraNanosecondsToAddToLastFetchedTimestamp = 0
	lastFetchedFileMode = 0644

	relChangelogFilepath = "/docs/changelog.md"

	// Taken from guess-release-version.sh
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
	checkArgs("<private key filepath>")
	privateKeyFilepath := os.Args[1]
	if 	_, err := os.Stat(privateKeyFilepath); err != nil {
		return stacktrace.Propagate(err, "An error occurred getting private key file.")
	}

	currentWorkingDirectory, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the current working directory.")
	}

	gitDirpath := path.Join(currentWorkingDirectory, gitDirname)
	if _, err := os.Stat(gitDirpath); err != nil {
		if os.IsNotExist(err) {
			return stacktrace.Propagate(err, "An error occurred getting the git repository in this directory. This means that this binary is not being run from root of a git repository.")
		}
	}

	repository, err := git.PlainOpen(currentWorkingDirectory)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to open the existing git repository.")
	}

	originRemote, err := repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	// Check local master branch exists
	localMasterBranch, err := repository.Branch(masterBranchName)
	if err != nil {
		return stacktrace.Propagate(err, "Missing required '%v' branch locally. Please run 'git checkout %v' Returned %+v.", masterBranchName, masterBranchName, localMasterBranch)
	}

	// Check no staged or unstaged changes exist on the branch before release
	worktree, err := repository.Worktree()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the worktree of the repository.")
	}

	currWorktreeStatus, err := worktree.Status()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the status of the worktree of the repository.")
	}

	isClean := currWorktreeStatus.IsClean()
	if !isClean {
		fmt.Printf("The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'\n", currWorktreeStatus)
		return nil
	}

	// Fetch remote if needed
	lastFetchedFilepath := path.Join(gitDirpath, lastFetchedFilename)
	shouldFetch := determineShouldFetch(lastFetchedFilepath)
	if shouldFetch {
		publicKeys, err := ssh.NewPublicKeysFromFile(gitUsername, privateKeyFilepath, emptyPassword)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred generating public key for authenticating fetch to origin master")
		}

		if err := originRemote.Fetch(&git.FetchOptions{Auth: publicKeys}); err != nil {
			return stacktrace.Propagate(err, "An error occurred fetching from the remote repository")
		}
		
		currentUnixTimeStr := fmt.Sprint(time.Now().Unix())
		if err := ioutil.WriteFile(lastFetchedFilepath, []byte(currentUnixTimeStr), lastFetchedFileMode); err != nil {
			return stacktrace.Propagate(err, "An error occurred writing last-fetched timestamp '%v' to file '%v'", currentUnixTimeStr, lastFetchedFilepath)
		}
	}

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

	isLocalMasterInSyncWithRemoteMaster := localMasterHash.String() == remoteMasterHash.String()
	if !isLocalMasterInSyncWithRemoteMaster {
		fmt.Println("The local master branch is not in sync with the remote master branch. Must be in sync to conduct release process.")
		return nil
	}
	
	// Guess the next release version
	latestReleaseVersion := getLatestReleaseVersion(repository, NO_PREVIOUS_VERSION)

	// Conduct changelog file validation
	changelogFilepath := path.Join(currentWorkingDirectory, relChangelogFilepath)

	tbdHeaderCount := grepFile(changelogFilepath, TBD_VERSION_HEADER_REGEX)
	if tbdHeaderCount != EXPECTED_NUM_TBD_HEADER_LINES {
		fmt.Printf("There should be %d TBD header lines in the changelog. Instead there are %d.\n", EXPECTED_NUM_TBD_HEADER_LINES, tbdHeaderCount)
		return nil
	}

	versionHeaderCount := grepFile(changelogFilepath, VERSION_HEADER_REGEX)
	if versionHeaderCount == 0 {
		fmt.Println("No previous changelog versions were detected in this changelog. Are you sure that the changelog is in sync with the release tags on this branch?")
		return nil
	}

	existsBreakingChanges := detectBreakingChanges(changelogFilepath)
	var nextReleaseVersion semver.Version
	if existsBreakingChanges {
		nextReleaseVersion = latestReleaseVersion.IncMinor()
	} else {
		nextReleaseVersion = latestReleaseVersion.IncPatch()
	}

	go wait()
	fmt.Printf("VERIFICATION: Release new version '%s'? (ENTER to continue, Ctrl-C to quit))", nextReleaseVersion.String())
	fmt.Scanln()

	return nil
}

func determineShouldFetch(lastFetchedFilepath string) bool {
	lastFetchedUnixTimeStr, err := ioutil.ReadFile(lastFetchedFilepath)
	if err != nil {
		return true
	}

	lastFetchedUnixTime, err := strconv.ParseUint(
		string(lastFetchedUnixTimeStr),
		lastFetchedTimestampUintParseBase,
		lastFetchedTimestampUintParseBits,
	)
	if err != nil {
		logrus.Errorf("An error occurred parsing last-fetch Unix time string '%v':\n%v", err)
	}
	lastFetchedTime := time.Unix(int64(lastFetchedUnixTime), extraNanosecondsToAddToLastFetchedTimestamp)
	noFetchNeededBefore := lastFetchedTime.Add(fetchGracePeriod)

	return time.Now().After(noFetchNeededBefore)
}

func getLatestReleaseVersion(repo *git.Repository, noPrevVersion string) semver.Version {
	semverRegex, err := regexp.Compile(SEMVER_REGEX)
	if err != nil {
		logrus.Errorf("Could not parse regexp: '%s'", SEMVER_REGEX, err)
	}

	tagrefs, err := repo.Tags()
	if err != nil {
		logrus.Errorf("An error occurred while retrieving tags for repository", err)
	}

	// Trim tagrefs and filter for only tags with X.Y.Z version format
	var tagSemVers []*semver.Version
	tagrefs.ForEach(func(tagref *plumbing.Reference) error {
		tagName := tagref.Name().String()
		tagName = strings.ReplaceAll(tagName, tagsPrefix, "")

		if semverRegex.Match([]byte(tagName)) {
			tagSemVer, err := semver.StrictNewVersion(tagName)
			if err != nil {
				logrus.Errorf("An error occurred while converting tags to semantic version.", err)
			}
			tagSemVers = append(tagSemVers, tagSemVer) 
		}
		return nil
	})

	var latestReleaseTagSemVer *semver.Version
	if len(tagSemVers) == 0 {
		latestReleaseTagSemVer, err = semver.StrictNewVersion(NO_PREVIOUS_VERSION)
		if err != nil {
			logrus.Errorf("An error occurred while converting tags to semantic version.", err)
		}
	} else {
		sort.Sort(sort.Reverse(semver.Collection(tagSemVers)))
		latestReleaseTagSemVer = tagSemVers[0]
	}

	return *latestReleaseTagSemVer
}

func detectBreakingChanges(changelogFilepath string) bool {
	changelogFile, err := os.Open(changelogFilepath);
	if err != nil {
		logrus.Errorf("Error attempting to open changelog file at provided path. Are you sure '%s' exists?", changelogFilepath, err)
	}
	defer changelogFile.Close()

	tbdRegex, err := regexp.Compile(TBD_VERSION_HEADER_REGEX)
	if err != nil {
		logrus.Errorf("Could not parse regexp: '%s'", TBD_VERSION_HEADER_REGEX, err)
	}
	breakingChangesRegex, err := regexp.Compile(BREAKING_CHANGES_SUBHEADER_REGEX)
	if err != nil {
		logrus.Errorf("Could not parse regexp: '%s'", BREAKING_CHANGES_SUBHEADER_REGEX, err)
	}
	versionHeaderRegex, err := regexp.Compile(VERSION_HEADER_REGEX)
	if err != nil {
		logrus.Errorf("Could not parse regexp: '%s'", VERSION_HEADER_REGEX, err)
	}

    scanner := bufio.NewScanner(changelogFile)

	// Find TBD header
    for scanner.Scan() {
		if tbdRegex.Match(scanner.Bytes()) {
			break
		}
    }

	// Scan file until next version header detected, searching for Breaking Changes header along the way
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
        logrus.Errorf("An error occurred while scanning file.\n", err)
    }
    return foundBreakingChanges
}

// adapted from: https://stackoverflow.com/questions/26709971/could-this-be-more-efficient-in-go
func grepFile(file string, regexPat string) int64 {
	r, err := regexp.Compile(regexPat)
	if err != nil {
		logrus.Errorf("Could not parse regexp: '%s'", regexPat, err)
	}
    patCount := int64(0)
    f, err := os.Open(file)
    if err != nil {
		logrus.Errorf("An error occurred while attempting to open file", err)
    }
    defer f.Close()
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
		if r.Match(scanner.Bytes()) {
			patCount++
		}
    }
    if err := scanner.Err(); err != nil {
        logrus.Errorf("An error occurred while scanning file.\n%v", err)
    }
    return patCount
}

func checkArgs(arg ...string){
	if len(os.Args) < len(arg)+1 {
		fmt.Printf("Usage: %s %s", os.Args[0], strings.Join(arg, " "))
		os.Exit(1)
	}
}

func wait() {
    i := 0
    for {
        time.Sleep(time.Second * 1)
        i++
    }
}