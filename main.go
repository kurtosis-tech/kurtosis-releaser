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
	// PHASE 0: Check that you’re in a currently git repository, return error if not
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

	// // cfg, err := repository.Config()
	// // fmt.Printf("Worktree: %s", cfg.Core.Worktree)

	_, err = repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	// // PHASE I: Check that local master branch is ready for a release
	// Check local master branch exists
	localMasterBranch, err := repository.Branch(masterBranchName)
	if err != nil {
		return stacktrace.Propagate(err, "Missing required '%v' branch locally. Please run 'git checkout %v'", masterBranchName, masterBranchName)
	}
	fmt.Printf("Local master branch: %+v\n", localMasterBranch)

	// // Check no staged or unstaged changes exist on the branch before release
	// worktree, err := repository.Worktree()
	// if err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred while trying to retrieve the worktree of the repository.")
	// }

	// currWorktreeStatus, err := worktree.Status()
	// if err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred while trying to retrieve the status of the worktree of the repository.")
	// }
	// fmt.Printf("Current working tree status: %s\n", currWorktreeStatus)

	// isClean := currWorktreeStatus.IsClean()
	// if !isClean {
	// 	fmt.Printf("The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'\n", currWorktreeStatus)
	// 	return nil
	// 	// return stacktrace.Propagate(err, "The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'", currWorktreeStatus)// no error here
	// }
	// fmt.Printf("Is working tree clean?: %t\n", isClean)

	// // Fetch remote origin master
	// // originRemote, err := repository.Remote(originRemoteName)
	// // if err != nil {
	// // 	return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	// // }

	// // sshPath := os.Getenv("HOME") + "/.ssh/id_ed25519"
	// // sshKey, _ := ioutil.ReadFile(sshPath)
	// // publicKey, keyError := ssh.NewPublicKeys("git", []byte(sshKey), "")
	// // if keyError != nil {
	// // 	fmt.Println(keyError)
	// // }
	// // fmt.Println("Attempting to fetch origin remote...")
	// // if err := originRemote.Fetch(&git.FetchOptions{}); err != nil {
	// // 	return stacktrace.Propagate(err, "An error occurred fetching remote '%v'.", originRemoteName)
	// // }
	// // fmt.Println("Fetched origin remote...")

	// // PHASE 2
	// //       - Check the last time remote master was fetched through `last-fetch-time.txt` in `.git`
	// //       - If remote master wasn’t fetched recently
	// //           - git fetch remote master branch
	// //           - if local master is not synced with remote master
	// //               - return error if so bc can’t call release on ancient version of master
	// //           - update `lasttime.txt`

	// // Check that local master and remote master are in sync
	// localMasterBranchName := masterBranchName
	// remoteMasterBranchName := fmt.Sprintf("%v/%v", originRemoteName, masterBranchName)

	// localMasterHash, err := repository.ResolveRevision(plumbing.Revision(localMasterBranchName))
	// if err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", localMasterBranchName)
	// }
	// remoteMasterHash, err := repository.ResolveRevision(plumbing.Revision(remoteMasterBranchName))
	// if err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", remoteMasterBranchName)
	// }

	// fmt.Println(localMasterHash.String())
	// fmt.Println(remoteMasterHash.String())
	// isLocalMasterInSyncWithRemoteMaster := localMasterHash.String() == remoteMasterHash.String()
	// fmt.Printf("Remote Master == Local Master?: %t\n", isLocalMasterInSyncWithRemoteMaster)
	// if !isLocalMasterInSyncWithRemoteMaster {
	// 	fmt.Println("The local master branch is not in sync with the remote master branch. Must be in sync to conduct release process.")
	// 	return nil
	// }

	// // PHASE 3
	// // - Guess the new release version
	// //   - get latest X.Y.Z version
	// //       - grab all tags on the branch
	// //       - filter for only tags with X.Y.Z version format
	// //       - sort and find latest
	// //   - look at changelog file to see if it contains `###Breaking Changes` header
	// //   - if yes: new release = X.(Y+1).0 else: X.Y.(Z+1)
	
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
	// fmt.Printf("TBD Header Count: %d\n", tbdHeaderCount)
	if tbdHeaderCount != EXPECTED_NUM_TBD_HEADER_LINES {
		fmt.Printf("There should be %d TBD header lines in the changelog. Instead there are %d.\n", EXPECTED_NUM_TBD_HEADER_LINES, tbdHeaderCount)
		return nil
	}


	versionHeaderCount, err := grepFile(changelogFilepath, VERSION_HEADER_REGEX)
	// fmt.Printf("Version Header Count: %d\n", versionHeaderCount)
	if versionHeaderCount == 0 {
		fmt.Println("No previous changelog versions were detected in this changelog. Are you sure that the changelog is in sync with the release tags on this branch?")
		return nil
	}

	// breakingChangesCount, err := grepFile(changelogFilepath, BREAKING_CHANGES_SUBHEADER_REGEX)
	// fmt.Printf("Breaking Changes Count: %d\n", breakingChangesCount)
	existsBreakingChanges, err := detectBreakingChanges(changelogFilepath, TBD_VERSION_HEADER_REGEX, BREAKING_CHANGES_SUBHEADER_REGEX, VERSION_HEADER_REGEX)
	if err!= nil {
		return stacktrace.Propagate(err, "Error occured while searching for breaking changes.")
	}

	fmt.Printf("Exists breaking changes: %t\n", existsBreakingChanges)
	var nextReleaseVersion semver.Version
	if existsBreakingChanges {
		nextReleaseVersion = latestReleaseVersion.IncMinor()
	} else {
		nextReleaseVersion = latestReleaseVersion.IncPatch()
	}

	fmt.Printf("Guessed Next Release Version: %s\n", nextReleaseVersion.String())

	fmt.Println("You made it to the end of the current releaser code!")
	return nil
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
        // if bytes.Contains(scanner.Bytes(), pat) {
        //     patCount++
        // }
		if r.Match(scanner.Bytes()) {
			// fmt.Println(string(scanner.Bytes()))
			patCount++
		}
    }
    if err := scanner.Err(); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
    return patCount, nil
}

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

	// for _, tagSemVer := range tagSemVers {
	// 	fmt.Println(tagSemVer.String())
	// }

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

func detectBreakingChanges(changelogFilepath string, tbdRegexString string, breakingChangesRegexString string, versionHeaderRegexString string) (bool, error) {
	changelogFile, err := os.Open(changelogFilepath);
	if err != nil {
		return false, stacktrace.Propagate(err, "Error attempting to open changelog file at provided path. Are you sure '%s' exists?", changelogFilepath)
	}
	defer changelogFile.Close()

	// fmt.Printf("detect breaking changes made it here %d\n", 1)

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

	// fmt.Printf("detect breaking changes made it here %d\n", 2)

    scanner := bufio.NewScanner(changelogFile)

	// Check for right amount of '# TBD' headers
    for scanner.Scan() {
		if tbdRegex.Match(scanner.Bytes()) {
			// fmt.Println("Found # TBD Header!")
			break
		}
    }

	// fmt.Printf("detect breaking changes made it here %d\n", 3)

	foundBreakingChanges := false
	for scanner.Scan() {
		// fmt.Println(string(scanner.Bytes()))

		if breakingChangesRegex.Match(scanner.Bytes()){
			// fmt.Println("Found Breaking Changes Header!")
			// fmt.Println(string(scanner.Bytes()))
			foundBreakingChanges = true
		}
		
		if versionHeaderRegex.Match(scanner.Bytes()){
			// fmt.Println("Found version header!")
			// fmt.Println(string(scanner.Bytes()))
			break
		}
	}

    if err := scanner.Err(); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
	// fmt.Printf("detect breaking changes made it here %d\n", 4)
    return foundBreakingChanges, nil
}