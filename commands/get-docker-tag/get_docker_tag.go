package getdockertag

import (
	"bytes"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/spf13/cobra"
	"os"
	"path"
	"regexp"
	"sort"
)

const (
	gitDirname         = ".git"
	abbrevCommitLength = 6
	dirtySuffix        = "-dirty"
	getDockerTagCmdStr = "get-docker-tag"

	// Rules on valid docker images: https://docs.docker.com/engine/reference/commandline/tag
	invalidDockerImgCharsRegexStr = "[^a-zA-Z0-9._-]|^\\.|^-"
)

var invalidDockerCharsRegex = regexp.MustCompile(invalidDockerImgCharsRegexStr)

var GetDockerTagCmd = &cobra.Command{
	Use:   getDockerTagCmdStr,
	Short: "Get Docker Image tag of repo",
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
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
	shouldAppendDirtySuffix := false
	worktree, err := repository.Worktree()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the worktree of the repository.")
	}
	currWorktreeStatus, err := worktree.Status()
	if err != nil {
		return stacktrace.Propagate(err, "An errorr occurred while trying to retrieve the status of the worktree of the repository.")
	}
	isClean := currWorktreeStatus.IsClean()
	if !isClean {
		shouldAppendDirtySuffix = true
	}

	// Get most recent commit
	head, err := repository.Head()
	mostRecentCommitHash := head.Hash()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred parsing revision '%v'", mostRecentCommitHash)
	}

	gitRef := ""
	// Get tag on most recent commit if it exists
	tag, err := getTagOnCommit(repository, mostRecentCommitHash)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred attempting to get tag on most recent commit '%s'", mostRecentCommitHash.String())
	}
	if tag != nil {
		gitRef = tag.Name().Short()
	}
	// If no tag exists, use abbreviated hash of most recent commit
	if gitRef == "" {
		abbrevCommitHash := mostRecentCommitHash.String()[0:abbrevCommitLength]
		gitRef = abbrevCommitHash
	}

	if shouldAppendDirtySuffix {
		gitRef = fmt.Sprintf("%s%s", gitRef, dirtySuffix)
	}

	// Sanitize gitref by replacing invalid docker image tag chars with _
	gitRef = invalidDockerCharsRegex.ReplaceAllString(gitRef, "_")

	dockerTag := gitRef
	fmt.Println(dockerTag)
	return nil
}

func getTagOnCommit(repo *git.Repository, commitHash plumbing.Hash) (*plumbing.Reference, error) {
	tagrefs, err := repo.Tags()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred attempting to get tags on this repository.")
	}

	var tags []*plumbing.Reference
	_ = tagrefs.ForEach(func(tagRef *plumbing.Reference) error {
		tagCommitHash, err := repo.ResolveRevision(plumbing.Revision(tagRef.Name().String()))
		if err != nil {
			return stacktrace.NewError("An error occurred resolving revision '%s'", tagRef.Name().String())
		}
		if bytes.Equal(commitHash[:], tagCommitHash[:]) {
			tags = append(tags, tagRef)
			// ErrStop is a go-git error type used to stop a a ForEach Iter
			return storer.ErrStop
		}
		return nil
	})
	if len(tags) > 0 {
		// sort tags alphabetically
		sort.SliceStable(tags, func(i int, j int) bool {
			return tags[i].Name().Short() < tags[j].Name().Short()
		})
		return tags[0], nil
	}

	return nil, nil
}
