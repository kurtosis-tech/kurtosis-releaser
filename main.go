package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"
)

const (
	gitDirname = ".git"

	// The name of the file inside the Git directory which will store when we last fetched (in Unix seconds)
	lastFetchedFilename = "last-fetch.txt"

	lastFetchedTimestampUintParseBase = 10
	lastFetchedTimestampUintParseBits = 64

	// How long we'll allow the user to go between fetches to ensure the repo is updated when they're releasing
	fetchGracePeriod = 30 * time.Second

	extraNanosecondsToAddToLastFetchedTimestamp = 0

	lastFetchedFileMode = 0644

	developBranchName = "develop"
	masterBranchName  = "master"

	originRemoteName = "origin"
)

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintln(logrus.StandardLogger().Out, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runMain() error {
	// Check if we're in the root of a Git repo
	currentWorkingDirpath, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the current working directory")
	}
	gitDirpath := path.Join(currentWorkingDirpath, gitDirname)
	if _, err := os.Stat(gitDirpath); err != nil {
		if os.IsNotExist(err); err != nil {
			return stacktrace.NewError("No Git directory found, meaning this isn't being run from the root of a Git repo")
		}
		return stacktrace.Propagate(err, "An unrecognized error occurred getting the Git directory at '%v'", gitDirpath)
	}
	repository, err := git.PlainOpen(currentWorkingDirpath)
	if err != nil {
		return stacktrace.Propagate(err, "Couldn't get a Git repo from the current repository")
	}
	originRemote, err := repository.Remote(originRemoteName)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting remote '%v' for repository; is the code pushed?", originRemoteName)
	}

	config.LoadConfig()

	// Fetch if needed
	lastFetchedFilepath := path.Join(gitDirpath, lastFetchedFilename)
	shouldFetch := determineShouldFetch(lastFetchedFilepath)
	if shouldFetch {
		if err := originRemote.Fetch(&git.FetchOptions{}); err != nil {
			return stacktrace.Propagate(err, "An error occurred fetching from the remote repository")
		}
		currentUnixTimeStr := fmt.Sprint(time.Now().Unix())
		if err := ioutil.WriteFile(lastFetchedFilepath, []byte(currentUnixTimeStr), lastFetchedFileMode); err != nil {
			return stacktrace.Propagate(err, "An error occurred writing last-fetched timestamp '%v' to file '%v'", currentUnixTimeStr, lastFetchedFilepath)
		}
	}

	// Compare that the local copies of
	/*
		developBranch, err := repository.Branch(developBranchName)
		if err != nil {
			return stacktrace.Propagate(err, "Missing required branch '%v' locally; you'll need to run 'git checkout %v'", developBranchName, developBranchName)
		}

		developBranch.Remote
	*/

	/*
		masterBranch, err := repository.Branch(masterBranchName)
		if err != nil {
			return stacktrace.Propagate(err, "Missing required branch '%v' locally; you'll need to run 'git checkout %v'", masterBranchName, masterBranchName)
		}
	*/

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
