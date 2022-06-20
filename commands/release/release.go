package release

import (
	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"	
	"github.com/spf13/cobra"

	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"regexp"
	"time"
	"strconv"
)

const (
	gitDirname = ".git"
	gitUsername = "git"
	originRemoteName = "origin"
	masterBranchName = "master"

	preReleaseScriptsFilename = ".pre-release-scripts.txt"
	relScriptsDirpath = "scripts"

	tagsPrefix = "refs/tags/"
	tagRefSpec = "refs/tags/*:refs/tags/*"
	headRef = "refs/heads/"

	// The name of the file inside the Git directory which will store when we last fetched (in Unix seconds)
	lastFetchedFilename = "last-fetch.txt"
	lastFetchedTimestampUintParseBase = 10
	lastFetchedTimestampUintParseBits = 64
	// How long we'll allow the user to go between fetches to ensure the repo is updated when they're releasing
	fetchGracePeriod = 1 * time.Minute
	extraNanosecondsToAddToLastFetchedTimestamp = 0
	lastFetchedFileMode = 0644

	relChangelogFilepath = "docs/changelog.md"
	changelogFileMode = 0644

	// Taken from guess-release-version.sh
	expectedNumTBDHeaderLines = 1
	tbdHeaderStr = "# TBD"
	noPreviousVersion = "0.0.0" 
	semverRegexStr = "^[0-9]+.[0-9]+.[0-9]+$"
	tbdHeaderRegexStr = "^#\\s*TBD\\s*$"
	versionHeaderRegexStr = "^#\\s*[0-9]+.[0-9]+.[0-9]+\\s*$"
	breakingChangesSubheaderRegexStr = "^###*\\s*[Bb]reak.*$"

	releaseCmdStr = "release"
)

var (
	semverRegex = regexp.MustCompile(semverRegexStr)
	tbdHeaderRegex = regexp.MustCompile(tbdHeaderRegexStr)
	versionHeaderRegex = regexp.MustCompile(versionHeaderRegexStr)
	breakingChangesRegex = regexp.MustCompile(breakingChangesSubheaderRegexStr)
)

var ReleaseCmd = &cobra.Command{
	Use: releaseCmdStr,
	Short: "Performs the release process for creating a new release of a versioned Kurtosis repo",
	RunE: run,
}

func run(cmd *cobra.Command, args []string) error {
	logrus.Infof("Starting release process...")
	currentWorkingDirpath, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the current working directory.")
	}
	gitDirpath := path.Join(currentWorkingDirpath, gitDirname)
	if _, err := os.Stat(gitDirpath); err != nil {
		if os.IsNotExist(err) {
			return stacktrace.Propagate(err, "An error occurred getting the git repository in this directory. This means that this binary is not being run from root of a git repository.")
		}
	}

	logrus.Infof("Retrieving git information...")
	repository, err := git.PlainOpen(currentWorkingDirpath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to open the existing git repository.")
	}
	globalRepoConfig, err := repository.ConfigScoped(config.GlobalScope)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to retrieve the global git config for this repo.")
	}
	name := globalRepoConfig.User.Name
	email := globalRepoConfig.User.Email
	if name == "" || email == "" {
		return stacktrace.NewError("The following empty name or email were detected in global git config: 'name: %s', 'email: %s'. Make sure these are set for annotating release commits.", name, email)
	}
	originRemote, err := repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	logrus.Infof("Conducting pre release checks...")
	worktree, err := repository.Worktree()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the worktree of the repository.")
	}

	// Check no staged or unstaged changes exist on the branch before release
	currWorktreeStatus, err := worktree.Status()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the status of the worktree of the repository.")
	}
	isClean := currWorktreeStatus.IsClean()
	if !isClean {
		return stacktrace.NewError("The branch contains modified files. Please ensure the working tree is clean before attempting to release. Currently the status is '%s'\n", currWorktreeStatus.String())
	}

	logrus.Infof("Fetching origin if needed...")
	// Fetch remote if needed
	lastFetchedFilepath := path.Join(gitDirpath, lastFetchedFilename)
	shouldFetch, err := determineShouldFetch(lastFetchedFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while determining if we should fetch from '%s'.", lastFetchedFilepath)
	}
	if shouldFetch {
		fetchOpts := &git.FetchOptions{RemoteName: originRemoteName}
		if err := originRemote.Fetch(fetchOpts); err != nil && err != git.NoErrAlreadyUpToDate {
			return stacktrace.Propagate(err, "An error occurred fetching from the remote repository.")
		}
		currentUnixTimeStr := fmt.Sprint(time.Now().Unix())
		if err := os.WriteFile(lastFetchedFilepath, []byte(currentUnixTimeStr), lastFetchedFileMode); err != nil {
			return stacktrace.Propagate(err, "An error occurred writing last-fetched timestamp '%v' to file '%v'.", currentUnixTimeStr, lastFetchedFilepath)
		}
	}

	logrus.Infof("Checking that %s and %s are in sync...", masterBranchName, originRemoteName)
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
		return stacktrace.NewError("The %s branch is not in sync with the %s branch. Must be in sync to conduct release process.", masterBranchName, originRemoteName)
	}

	logrus.Infof("Checking out %s branch...", masterBranchName)
	masterBranchRef := plumbing.ReferenceName(fmt.Sprintf("%s%s", headRef, masterBranchName))
	err = worktree.Checkout(&git.CheckoutOptions{Branch: masterBranchRef})
	if err != nil {
		return stacktrace.Propagate(err, "Missing required '%v' branch locally. Please run 'git checkout %v'", masterBranchName, masterBranchName)
	}
	
	// Conduct changelog file validation
	changelogFilepath := path.Join(currentWorkingDirpath, relChangelogFilepath)
	tbdHeaderCount, err := grepFile(changelogFilepath, tbdHeaderRegex)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read the number of lines in '%s' matching the following regex: '%s'.", changelogFilepath, tbdHeaderRegex.String())	
	}
	if tbdHeaderCount != expectedNumTBDHeaderLines {
		return stacktrace.NewError("There should be %d TBD header lines in the changelog. Instead there are %d.\n", expectedNumTBDHeaderLines, tbdHeaderCount)
	}
	versionHeaderCount, err := grepFile(changelogFilepath, versionHeaderRegex)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to read the number of lines in '%s' matching the following regex: '%s'.", changelogFilepath, versionHeaderRegex.String())	
	}
	if versionHeaderCount == 0 {
		return stacktrace.NewError("No previous release versions were detected in this changelog. Are you sure that the changelog is in sync with the release tags on this branch?")
	}
	logrus.Infof("Finished prererelease checks.")

	logrus.Infof("Guessing next release version...")
	latestReleaseVersion, err := getLatestReleaseVersion(repository)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the latest release version.")
	}
	existsBreakingChanges, err := doBreakingChangesExist(changelogFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while detecting if breaking changes exist.")
	}
	var nextReleaseVersion semver.Version
	if existsBreakingChanges {
		nextReleaseVersion = latestReleaseVersion.IncMinor()
	} else {
		nextReleaseVersion = latestReleaseVersion.IncPatch()
	}

	logrus.Infof("VERIFICATION: Release new version '%s'? (ENTER to continue, Ctrl-C to quit)", nextReleaseVersion.String())
	_, err = fmt.Scanln()
	if err != nil {
		return nil
	}

	shouldResetLocalBranch := true
	defer func() {
		if shouldResetLocalBranch {
			// git reset --hard origin/master
			err = worktree.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: *remoteMasterHash })
			if err != nil {
				logrus.Errorf("ACTION REQUIRED: Error occurred attempting to undo local changes made for release '%s'. Please run 'git reset --hard %s' to undo manually.", nextReleaseVersion.String(), remoteMasterBranchName, err)
			}
		}
	}()

	logrus.Infof("Running prerelease scripts...")
	preReleaseScriptsDirpath := path.Join(currentWorkingDirpath, relScriptsDirpath)
	err = runPreReleaseScripts(preReleaseScriptsDirpath, nextReleaseVersion.String())
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while running prerelease scripts.")
	}

	logrus.Infof("Updating the changelog...")
	err = updateChangelog(changelogFilepath, nextReleaseVersion.String())
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while updating the changelog.")
	}

	logrus.Infof("Committing changes locally...")
	// Commit pre release changes
	err = worktree.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to add release changes to staging area.")
	}
	commitMsg := fmt.Sprintf("Finalize changes for release version '%s'", nextReleaseVersion.String())
	_, err = worktree.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
	})

	logrus.Infof("Setting next release version tag...")
	// Set next release version tag
	releaseTag := nextReleaseVersion.String()
	vReleaseTag := fmt.Sprintf("v%s", nextReleaseVersion.String())
	head, err := repository.Head()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to get the ref to HEAD of the local repository.")
	}
	_, err = repository.CreateTag(releaseTag, head.Hash(), &git.CreateTagOptions{
		Message: releaseTag,
	})
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to create this git tag for the next release version: %s.", releaseTag)
	}
	shouldUndoReleaseTag := true
	defer func() {
		if shouldUndoReleaseTag {
			// git tag -d
			err = repository.DeleteTag(releaseTag)
			if err != nil {
				logrus.Errorf("ACTION REQUIRED: An error occurred attempting to undo tag '%s'. Please run 'git tag -d %s' to delete the tag manually.", releaseTag, err)
			}
		}
	}()
	_, err = repository.CreateTag(vReleaseTag, head.Hash(), &git.CreateTagOptions{
		Message: vReleaseTag,
	})
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to create this git tag for the next release version: %s.", vReleaseTag)
	}
	shouldUndoVPrefixedReleaseTag := true
	defer func() {
		if shouldUndoVPrefixedReleaseTag {
			// git tag -d
			err = repository.DeleteTag(vReleaseTag)
			if err != nil {
				logrus.Errorf("ACTION REQUIRED: An error occurred attempting to undo tag '%s'. Please run 'git tag -d %s' to delete the tag manually.", vReleaseTag, vReleaseTag, err)
			}
		}
	}()

	logrus.Infof("Pushing release changes to '%s'...", remoteMasterBranchName)
	pushCommitOpts := &git.PushOptions{RemoteName: originRemoteName}
	if err = repository.Push(pushCommitOpts); err != nil {
		return stacktrace.Propagate(err, "An error occurred while pushing release changes to '%s'.", remoteMasterBranchName)
	}
	shouldWarnAboutUndoingRemotePush := true
	defer func() {
		if shouldWarnAboutUndoingRemotePush {
			logrus.Errorf("ACTION REQUIRED: An error occurred meaning we need to undo our push-to-%s, but this is a dangerous operation for its risk that it will destroy history on the remote so you'll need to do this manually. Follow this tutorial: 'LINK TO INSTRUCTIONS TO UNDO PUSH.'", originRemoteName, err)
		}
	}()

	shouldResetLocalBranch = false
	shouldUndoReleaseTag = false
	shouldUndoVPrefixedReleaseTag = false

	logrus.Infof("Pushing release tags to '%s'...", remoteMasterBranchName) 
	releaseTagRefSpec := fmt.Sprintf("refs/tags/%s:refs/tags/%s", releaseTag, releaseTag) 
	pushReleaseTagOpts := &git.PushOptions{
		RemoteName: originRemoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec(releaseTagRefSpec)},
	}
	if 	err = repository.Push(pushReleaseTagOpts); err != nil {
		return stacktrace.Propagate(err, "An error occurred while pushing release tag: '%s' to '%s'.", releaseTag, remoteMasterBranchName)
	}
	// Best effort push of the v prefixed tag
	vReleaseTagRefSpec := fmt.Sprintf("refs/tags/%s:refs/tags/%s", vReleaseTag, vReleaseTag) 
	pushVPrefixedReleaseTagOpts := &git.PushOptions{
		RemoteName: originRemoteName,
		RefSpecs:   []config.RefSpec{config.RefSpec(vReleaseTagRefSpec)},
	}
	if 	err = repository.Push(pushVPrefixedReleaseTagOpts); err != nil {
		logrus.Errorf("An error occurred while pushing release tag: '%s' to '%s'.", vReleaseTag, remoteMasterBranchName, err)
	}

	shouldWarnAboutUndoingRemotePush = false

	logrus.Infof("Release success.")
	return nil
}

// ====================================================================================================
//                                       Private Helper Functions
// ====================================================================================================
func determineShouldFetch(lastFetchedFilepath string) (bool, error) {
	lastFetchedUnixTimeStr, err := os.ReadFile(lastFetchedFilepath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Errorf("An error occurred opening the file to determine fetching at '%s'..", lastFetchedFilepath, err)
			return true, nil
		}
		return false, stacktrace.Propagate(err, "An error occurred reading the file to determine fetching:'%s'.", lastFetchedFilepath)
	}

	lastFetchedUnixTime, err := strconv.ParseUint(
		string(lastFetchedUnixTimeStr),
		lastFetchedTimestampUintParseBase,
		lastFetchedTimestampUintParseBits,
	)
	if err != nil {
		return false, stacktrace.Propagate(err, "An error occurred parsing last-fetch Unix time string: '%v'.", lastFetchedUnixTimeStr)
	}
	lastFetchedTime := time.Unix(int64(lastFetchedUnixTime), extraNanosecondsToAddToLastFetchedTimestamp)
	noFetchNeededBefore := lastFetchedTime.Add(fetchGracePeriod)

	return time.Now().After(noFetchNeededBefore), nil
}

func getLatestReleaseVersion(repo *git.Repository) (*semver.Version, error) {
	tagrefs, err := repo.Tags()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred while retrieving tags for repository.")
	}

	// Trim tagrefs and filter for only tags with X.Y.Z version format
	var allTagSemVers []*semver.Version
	err = tagrefs.ForEach(func(tagref *plumbing.Reference) error {
		tagName := tagref.Name().String()
		tagName = strings.ReplaceAll(tagName, tagsPrefix, "")

		if semverRegex.Match([]byte(tagName)) {
			tagSemVer, err := semver.StrictNewVersion(tagName)
			if err != nil {
				return stacktrace.Propagate(err, "An error occurred while retrieving the following tag: %s.", tagName)
			}
			allTagSemVers = append(allTagSemVers, tagSemVer) 
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var latestReleaseTagSemVer *semver.Version
	if len(allTagSemVers) == 0 {
		latestReleaseTagSemVer, err = semver.StrictNewVersion(noPreviousVersion)
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred creating '%s' semantic version.", noPreviousVersion)
		}
	} else {
		sort.Sort(sort.Reverse(semver.Collection(allTagSemVers)))
		latestReleaseTagSemVer = allTagSemVers[0]
	}

	return latestReleaseTagSemVer, nil
}

func doBreakingChangesExist(changelogFilepath string) (bool, error) {
	changelogFile, err := os.Open(changelogFilepath)
	if err != nil {
		return false, stacktrace.Propagate(err, "An error occurred attempting to open changelog file at provided path. Are you sure '%s' exists?", changelogFilepath)
	}
	defer changelogFile.Close()

    scanner := bufio.NewScanner(changelogFile)
	// Find TBD header
    for scanner.Scan() {
		if tbdHeaderRegex.Match(scanner.Bytes()) {
			break
		}
    }
	if err := scanner.Err(); err != nil {
        return false, stacktrace.Propagate(err, "An error occurred while scanning for the TBD header in the changelog file at provided path: '%s'.\n", changelogFilepath)
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
		return false, stacktrace.Propagate(err, "An error occurred while scanning for the breaking changes header in the changelog file at provided path: '%s'.\n", changelogFilepath)
    }

    return foundBreakingChanges, nil
}

func runPreReleaseScripts(preReleaseScriptsDirpath string, releaseVersion string) error {
	preReleaseScriptsFilepath := path.Join(preReleaseScriptsDirpath, preReleaseScriptsFilename)
	preReleaseScriptsFile, err := os.ReadFile(preReleaseScriptsFilepath);
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to open file at provided path. Are you sure '%s' exists?", preReleaseScriptsFilepath)
	}

	lines := bytes.Split(preReleaseScriptsFile, []byte("\n"))
	var allScriptFilepaths []string
	for _, line := range(lines) {
		allScriptFilepaths = append(allScriptFilepaths, string(line))
	}
	for _, scriptFilepath := range allScriptFilepaths {
		if strings.TrimSpace(scriptFilepath) == "" {
			continue
		}
		scriptCmdString := path.Join(preReleaseScriptsDirpath, scriptFilepath)
		scriptCmd := exec.Command(scriptCmdString, releaseVersion)
		err := scriptCmd.Run()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred attempting to run the following pre release script command: '%s %s'", scriptCmdString, releaseVersion)
		}
	}

	return nil
}

func updateChangelog(changelogFilepath string, releaseVersion string) error {
	changelogFile, err := os.ReadFile(changelogFilepath)
	if err != nil {
		return stacktrace.Propagate(err, "An error attempting to open changelog file at provided path. Are you sure '%s' exists?", changelogFilepath)
	}

	lines := bytes.Split(changelogFile, []byte("\n"))
	newLines:= make([][]byte, len(lines) + 2) 
	// Add a new TBD header for next release
	newLines[0] = []byte(tbdHeaderStr)
	i := 1
	for _, line := range lines {
		// Change current TBD header to Release Version header
		if tbdHeaderRegex.Match(line){
			releaseVersionHeader := fmt.Sprintf("# %s", releaseVersion)
			newLines[i + 1] = []byte(releaseVersionHeader)
			i = i + 2
		} else {
			newLines[i] = line
			i++
		}
	}
	newChangelogFile := bytes.Join(newLines, []byte("\n"))
	err = os.WriteFile(changelogFilepath, newChangelogFile, changelogFileMode)
	if err != nil {
		return stacktrace.Propagate(err, "An error attempting to write changelog file to '%s'.", changelogFilepath)
	}

	return nil
}

// adapted from: https://stackoverflow.com/questions/26709971/could-this-be-more-efficient-in-go
func grepFile(filePath string, regexPat *regexp.Regexp) (int64, error) {
    numLinesMatchingPattern := int64(0)
    file, err := os.Open(filePath)
    if err != nil {
		return -1, stacktrace.Propagate(err, "An error occurred while attempting to open file: '%s'.", filePath)
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
		if regexPat.Match(scanner.Bytes()) {
			numLinesMatchingPattern++
		}
    }
    if err := scanner.Err(); err != nil {
		return -1, stacktrace.Propagate(err, "An error occurred while scanning file: '%s'.", filePath)
    }
    return numLinesMatchingPattern, nil
}