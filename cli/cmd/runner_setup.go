package cmd

import (
	"fmt"

	"github.com/impishMD/taskexec/cli/setup"
	"github.com/impishMD/taskexec/util"
	"github.com/spf13/cobra"
)

func init() {
	runnerCmd.AddCommand(runnerSetupCmd)
}

var runnerSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Perform interactive setup",
	Run: func(cmd *cobra.Command, args []string) {
		doRunnerSetup()
	},
}

// nolint: gocyclo
func doRunnerSetup() int {
	config := &util.ConfigType{}

	setup.InteractiveRunnerSetup(config)
	resultConfigPath := setup.SaveConfig(config, "config.runner.json", persistentFlags.configPath)
	util.ConfigInit(resultConfigPath, false)

	if util.Config.Runner.RegistrationToken == "" && config.Runner.RegistrationToken != "" {
		util.Config.Runner.RegistrationToken = config.Runner.RegistrationToken
	}

	if util.Config.Runner.RegistrationToken != "" {
		taskPool := createRunnerJobPool()
		err := taskPool.Register(&resultConfigPath)
		if err != nil {
			panic(err)
		}
	}

	fmt.Printf(" Re-launch this program pointing to the configuration file\n\n./taskexec runner start --config %v\n\n", resultConfigPath)
	fmt.Printf(" To run as daemon:\n\nnohup ./taskexec runner start --config %v &\n\n", resultConfigPath)

	return 0
}
