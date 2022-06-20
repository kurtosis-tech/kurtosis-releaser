package getdockertag

import (
	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"	
	"regexp"
)

const (
	gitDirname = ".git"
	gitUsername = "git"
	originRemoteName = "origin"
	masterBranchName = "master"

	tagsPrefix = "refs/tags/"
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

	appendDirtySuffix := false
	// Check no staged or unstaged changes exist on the branch before release
	currWorktreeStatus, err := worktree.Status()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred while trying to retrieve the status of the worktree of the repository.")
	}
	isClean := currWorktreeStatus.IsClean()
	if !isClean {
		appendDirtySuffix = true
	}
	
	tagrefs, err := repository.Tags()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred while retrieving tags for repository.")
	}
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

	// Get latest tag if it exists
	gitRef := nil
	if len(allTagSemVers) > 0 {
		sort.Sort(sort.Reverse(semver.Collection(allTagSemVers)))
		gitRef = allTagSemVers[0].String()
	}

	// If tag doesn't exist, get branch name
	if gitRef := nil {

	}

	// Trim tagrefs and filter for only tags with X.Y.Z version format

	return nil
}
