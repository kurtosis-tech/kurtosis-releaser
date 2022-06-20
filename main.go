package main

import (
	"github.com/kurtosis-tech/kurtosis-releaser/commands"
	"github.com/sirupsen/logrus"
	"os"
)


func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})

	if err := commands.RootCmd.Execute(); err != nil {
		// We don't actually need to print the error because Cobra will do it for us
		os.Exit(1)
	}
	os.Exit(0)
}