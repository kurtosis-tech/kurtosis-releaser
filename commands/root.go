package commands

import (
	"github.com/kurtosis-tech/kurtosis-releaser/commands/get-docker-tag"
	"github.com/kurtosis-tech/kurtosis-releaser/commands/release"
	"github.com/kurtosis-tech/stacktrace"
	"github.com/sirupsen/logrus"	
	"github.com/spf13/cobra"
	"strings"
)

const (
	cliCmdStr = "kurtosis-releaser <action>"
	cliLogLevelStrFlag = "cli-log-level"
)

var RootCmd = &cobra.Command{
	Use:   cliCmdStr,
	Short: "A CLI with essential tools for Kurtosis developers.",

	// Cobra will print usage whenever _any_ error occurs, including ones we throw in Kurtosis
	// This doesn't make sense in 99% of the cases, so just turn them off entirely
	SilenceUsage:      true,
	PersistentPreRunE: globalSetup,
}

var logLevelStr string
var defaultLogLevelStr = logrus.InfoLevel.String()
  
func init(){
	RootCmd.PersistentFlags().StringVar(
		&logLevelStr,
		cliLogLevelStrFlag,
		defaultLogLevelStr,
		"Sets the level that the CLI will log at ("+strings.Join(GetAcceptableLogLevelStrs(), "|")+")",
	)

	RootCmd.AddCommand(release.ReleaseCmd)
	RootCmd.AddCommand(getdockertag.GetDockerTagCmd)
}

// ====================================================================================================
//                                       Private Helper Functions
// ====================================================================================================
func globalSetup(cmd *cobra.Command, args []string) error {
	if err := setupCLILogs(cmd); err != nil {
		return stacktrace.Propagate(err, "An error occurred setting up CLI logs")
	}
	return nil
}

func setupCLILogs(cmd *cobra.Command) error {
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		return stacktrace.Propagate(err, "Could not parse log level string '%v'", logLevelStr)
	}
	logrus.SetOutput(cmd.OutOrStdout())
	logrus.SetLevel(logLevel)
	return nil
}

func GetAcceptableLogLevelStrs() []string {
	result := []string{}
	for _, level := range logrus.AllLevels {
		levelStr := level.String()
		result = append(result, levelStr)
	}
	return result
}