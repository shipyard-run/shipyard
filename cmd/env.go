package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/shipyard"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/spf13/cobra"
)

func newEnvCmd(e shipyard.Engine) *cobra.Command {
	var unset bool

	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Prints environment variables defined by the blueprint",
		Long:  "Prints environment variables defined by the blueprint",
		Example: `
  # Display environment variables
  shipyard env
  
  VAR1=value
  VAR2=value
  
  # Set environment variables on Linux based systems
  eval $(shipyard env)
    
  # Set environment variables on Windows based systems
  Invoke-Expression "shipyard env" | ForEach-Object { Invoke-Expression $_ }

  # Unset environment variables on Linux based systems
  eval $(shipyard env --unset)

  # Unset environment variables on Windows based systems
  Invoke-Expression "shipyard env --unset" | ForEach-Object { Remove-Item $_ }
`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			c := config.New()
			err := c.FromJSON(utils.StatePath())
			if err != nil {
				fmt.Println("Unable to load state", err)
				os.Exit(1)
			}

			prefix := "export "
			if unset {
				prefix = "unset "
			}
			if runtime.GOOS == "windows" {
				prefix = "$Env:"
				if unset {
					prefix = "Env:\\"
				}
			}

			if c.Blueprint != nil && len(c.Blueprint.Environment) > 0 {
				for _, env := range c.Blueprint.Environment {
					env.Value = strings.ReplaceAll(env.Value, `\`, `\\`)
					if unset {
						fmt.Printf("%s%s\n", prefix, env.Key)
					} else {
						fmt.Printf("%s%s=\"%s\"\n", prefix, env.Key, env.Value)
					}
				}
			}

			// add output variables
			for _, r := range c.Resources {
				if r.Info().Type == config.TypeOutput {
					val := strings.ReplaceAll(r.(*config.Output).Value, `\`, `\\`)
					if unset {
						fmt.Printf("%s%s\n", prefix, r.Info().Name)
					} else {
						fmt.Printf("%s%s=\"%s\"\n", prefix, r.Info().Name, val)
					}
				}
			}
			return nil
		},
		SilenceUsage: true,
	}

	envCmd.Flags().BoolVarP(&unset, "unset", "", false, "When set to true Shipyard will print unset commands for environment variables defined by the blueprint")
	return envCmd
}
