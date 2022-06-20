package getdockertag

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	gitDirname = ".git"
	gitUsername = "git"
	originRemoteName = "origin"
	masterBranchName = "master"

	tagsPrefix = "refs/tags/"
	headRef = "refs/heads/"
	dirtySuffix = "-dirty"
	getDockerTagCmdStr = "get-docker-tag"
	semverRegexStr = "^[0-9]+.[0-9]+.[0-9]+$"

	gitRefSanitizingRegexStr = "s,[/:],_,g"
)

var semverRegex = regexp.MustCompile(semverRegexStr)
var gitRefSanitizingRegex = regexp.MustCompile(gitRefSanitizingRegexStr)

var GetDockerTagCmd = &cobra.Command{
	Use: getDockerTagCmdStr,
	Short: "Prints the expected docker tag given the current state of the Kurtosis repo this command is run on.",
	RunE: run,
}

func run(cmd *cobra.Command, args []string) error {
	logrus.Infof("Retrieving git information...")
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
	repository, err := git.PlainOpen(currentWorkingDirpath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while attempting to open the existing git repository.")
	}

	// Determines if working tree is clean
	appendDirtySuffix := false
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
		appendDirtySuffix = true
	}

	// Get the latest tag in the repo, if it exists
	tagrefs, err := repository.Tags()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while retrieving tags for repository.")
	}
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
		return err
	}

	// Get latest tag, if it exists
	gitRef := ""
	if len(allTagSemVers) > 0 {
		sort.Sort(sort.Reverse(semver.Collection(allTagSemVers)))
		gitRef = allTagSemVers[0].String()
	}

	// If no tags exist, get branch name
	if gitRef == "" {
		head, err := repository.Head()
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred while attempting to get the ref to HEAD of the local repository.")
		}
		branchRefStr := head.Name().String()
		branchName := strings.ReplaceAll(branchRefStr, headRef, "")
		gitRef = branchName
	}

	// add dirty if needed
	if appendDirtySuffix {
		gitRef = fmt.Sprintf("%s%s", gitRef, dirtySuffix)
	}

	// Sanitize gitref for docker tag by replacing all ':' or '/' characters with '_'
	gitRef = strings.ReplaceAll(gitRef, ":", "_")
	gitRef = strings.ReplaceAll(gitRef, "/", "_")

	dockerTag := gitRef
	logrus.Infof(dockerTag)
	return nil
}