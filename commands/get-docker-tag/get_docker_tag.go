package getdockertag

import (
	"bytes"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	gitDirname               = ".git"
	gitRefSanitizingRegexStr = "s,[/:],_,g"
	masterBranchName         = "master"
	dirtySuffix              = "-dirty"
	getDockerTagCmdStr       = "get-docker-tag"
	semverRegexStr           = "^[0-9]+.[0-9]+.[0-9]+$"
)

var semverRegex = regexp.MustCompile(semverRegexStr)
var gitRefSanitizingRegex = regexp.MustCompile(gitRefSanitizingRegexStr)

var GetDockerTagCmd = &cobra.Command{
	Use:   getDockerTagCmdStr,
	Short: "Prints the expected docker tag given the current state of the Kurtosis repo this command is run on.",
	RunE:  run,
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

	// Get most recent commit
	localMasterHash, err := repository.ResolveRevision(plumbing.Revision(masterBranchName))
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", masterBranchName)
	}

	gitRef := ""
	// Get tag on most recent commit if it exists
	tag, err := getTagOnCommit(repository, localMasterHash)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to get tag on most recent commit '%s'", localMasterHash.String())
	}
	if tag != nil {
		gitRef = tag.Name().Short()
	}
	// If no tag exists, use abbreviated hash of most recent commit
	if gitRef == "" {
		abbrevCommitHash := localMasterHash.String()[0:6]
		gitRef = abbrevCommitHash
	}

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

func getTagOnCommit(repo *git.Repository, commitHash *plumbing.Hash) (*plumbing.Reference, error) {
	tagrefs, err := repo.Tags()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred attempting to get tags on this repository.")
	}

	var tag *plumbing.Reference
	_ = tagrefs.ForEach(func(tagRef *plumbing.Reference) error {
		tagCommitHash, err := repo.ResolveRevision(plumbing.Revision(tagRef.Name().String()))
		if err != nil {
			return stacktrace.NewError("An error occurred resolving revision '%s'", tagRef.Name().String())
		}
		if bytes.Equal(commitHash[:], tagCommitHash[:]) && semverRegex.Match([]byte(tagRef.Name().Short())) {
			tag = tagRef
			return storer.ErrStop
		}
		return nil
	})

	return tag, nil
}
