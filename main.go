package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

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
	changelogFilepath = "/docs/changelog.md"
	VERSION_HEADER_REGEX="^#[[:space:]]*[0-9]+\.[0-9]+\.[0-9]+[[:space:]]*$"
	BREAKING_CHANGES_SUBHEADER_REGEX='^'"${HEADER_CHAR}${HEADER_CHAR}${HEADER_CHAR}"'*[[:space:]]*[Bb]reak.*$'
	TBD_VERSION_HEADER_REGEX='^'"${HEADER_CHAR}"'[[:space:]]*TBD[[:space:]]*$'
	HEADER_CHAR="#"
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

	// cfg, err := repository.Config()
	// fmt.Printf("Worktree: %s", cfg.Core.Worktree)

	_, err = repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	// PHASE I: Check that local master branch is ready for a release
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
		// return stacktrace.Propagate(err, "The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'", currWorktreeStatus)// no error here
	}
	fmt.Printf("Is working tree clean?: %t\n", isClean)

	// Fetch remote origin master
	// originRemote, err := repository.Remote(originRemoteName)
	// if err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	// }

	// sshPath := os.Getenv("HOME") + "/.ssh/id_ed25519"
	// sshKey, _ := ioutil.ReadFile(sshPath)
	// publicKey, keyError := ssh.NewPublicKeys("git", []byte(sshKey), "")
	// if keyError != nil {
	// 	fmt.Println(keyError)
	// }
	// fmt.Println("Attempting to fetch origin remote...")
	// if err := originRemote.Fetch(&git.FetchOptions{}); err != nil {
	// 	return stacktrace.Propagate(err, "An error occurred fetching remote '%v'.", originRemoteName)
	// }
	// fmt.Println("Fetched origin remote...")

	// PHASE 2
	//       - Check the last time remote master was fetched through `last-fetch-time.txt` in `.git`
	//       - If remote master wasn’t fetched recently
	//           - git fetch remote master branch
	//           - if local master is not synced with remote master
	//               - return error if so bc can’t call release on ancient version of master
	//           - update `lasttime.txt`

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

	// PHASE 3
	// - Guess the new release version
	//   - get current X.Y.Z version
	//       - grab all tags on the branch
	//       - filter for only tags with X.Y.Z version format
	//       - sort and find latest
	//   - look at changelog file to see if it contains `###Breaking Changes` header
	//   - if yes: new release = X.(Y+1).0 else: X.Y.(Z+1)
	
	// Grab all tags on the branch
	tagrefs, err := repository.Tags()

	// Filter for only tags with X.Y.Z version format
	var tagSemVers []*semver.Version
	err = tagrefs.ForEach(func(tagref *plumbing.Reference) error {
		tagName := tagref.Name().String()
		tagName = strings.ReplaceAll(tagName, tagsPrefix, "")
		tagSemVer, err := semver.StrictNewVersion(tagName)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred while attempting to parse semantic version of tag.")
		}
		tagSemVers = append(tagSemVers, tagSemVer) 
		return nil
	})

	// Sort 
	sort.Sort(sort.Reverse(semver.Collection(tagSemVers)))

	// for _, tagSemVer := range tagSemVers {
	// 	fmt.Println(tagSemVer.String())
	// }

	// Retrieve latest tag
	latestReleaseTagSemVer := tagSemVers[0]
	fmt.Println(latestReleaseTagSemVer)

	// Look at changelog file to see if it contains `###Breaking Changes` header

	
	fmt.Println("You made it to the end of the current releaser code!")
	return nil
}
