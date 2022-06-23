package sed

import "github.com/spf13/cobra"

const (
	sedCmdStr = "sed"
)

var SedCmd = &cobra.Command{
	Use:   sedCmdStr,
	Short: "Performs the release process for creating a new release of a versioned Kurtosis repo",
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	return nil
}
