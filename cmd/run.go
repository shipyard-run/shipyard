package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	gvm "github.com/shipyard-run/version-manager"

	"github.com/shipyard-run/shipyard/pkg/clients"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/shipyard"
	"github.com/shipyard-run/shipyard/pkg/utils"
	"github.com/spf13/cobra"

	markdown "github.com/MichaelMure/go-term-markdown"
)

func newRunCmd(e shipyard.Engine, bp clients.Getter, hc clients.HTTP, bc clients.System, vm gvm.Versions, cc clients.Connector, l hclog.Logger) *cobra.Command {
	var noOpen bool
	var force bool
	var y bool
	var runVersion string
	var variables []string
	var variablesFile string

	runCmd := &cobra.Command{
		Use:   "run [file] [directory] ...",
		Short: "Run the supplied stack configuration",
		Long:  `Run the supplied stack configuration`,
		Example: `
  # Recursively create a stack from a directory
  shipyard run ./-stack

  # Create a stack from a specific file
  shipyard run my-stack/network.hcl

  # Create a stack from a blueprint in GitHub
  shipyard run github.com/shipyard-run/blueprints//vault-k8s
	`,
		Args:         cobra.ArbitraryArgs,
		RunE:         newRunCmdFunc(e, bp, hc, bc, vm, cc, &noOpen, &force, &runVersion, &y, &variables, &variablesFile, l),
		SilenceUsage: true,
	}

	runCmd.Flags().StringVarP(&runVersion, "version", "v", "", "When set, run creates the specified resources using a particular Shipyard version")
	runCmd.Flags().BoolVarP(&y, "y", "y", false, "When set, Shipyard will not prompt for confirmation")
	runCmd.Flags().BoolVarP(&noOpen, "no-browser", "", false, "When set to true Shipyard will not open the browser windows defined in the blueprint")
	runCmd.Flags().BoolVarP(&force, "force-update", "", false, "When set to true Shipyard ignores cached images or files and will download all resources")
	runCmd.Flags().StringSliceVarP(&variables, "var", "", nil, "Allows setting variables from the command line, variables are specified as a key and value, e.g --var key=value. Can be specified multiple times")
	runCmd.Flags().StringVarP(&variablesFile, "vars-file", "", "", "Load variables from a location other than *.vars files in the blueprint folder. E.g --vars-file=./file.vars")

	return runCmd
}

func newRunCmdFunc(e shipyard.Engine, bp clients.Getter, hc clients.HTTP, bc clients.System, vm gvm.Versions, cc clients.Connector, noOpen *bool, force *bool, runVersion *string, autoApprove *bool, variables *[]string, variablesFile *string, l hclog.Logger) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// create the shipyard and sub folders in the users home directory
		utils.CreateFolders()

		if *force == true {
			bp.SetForce(true)
			e.GetClients().ContainerTasks.SetForcePull(true)
			e.GetClients().Getter.SetForce(true)
		}

		// parse the vars into a map
		vars := map[string]string{}
		for _, v := range *variables {
			parts := strings.Split(v, "=")
			if len(parts) == 2 {
				vars[parts[0]] = parts[1]
			}
		}

		// Check the system to see if Docker is running and everything is installed
		s, err := bc.Preflight()
		if err != nil {
			cmd.Println("")
			cmd.Println("###### SYSTEM DIAGNOSTICS ######")
			cmd.Println(s)
			return err
		}

		// check the variables file exists
		if variablesFile != nil && *variablesFile != "" {
			if _, err := os.Stat(*variablesFile); err != nil {
				return fmt.Errorf("Variables file %s, does not exist", *variablesFile)
			}
		} else {
			vf := ""
			variablesFile = &vf
		}

		// are we running with a different shipyard version, if so check it is installed
		if *runVersion != "" {
			return runWithOtherVersion(*runVersion, *autoApprove, args, *force, *noOpen, cmd, vm, bc, *variables, *variablesFile)
		}

		// create the certificates for the connector
		if cb, err := cc.GetLocalCertBundle(utils.CertsDir("")); err != nil || cb == nil {
			// generate certs
			l.Debug("Generating TLS Certificates for Ingress", "path", utils.CertsDir(""))
			_, err := cc.GenerateLocalCertBundle(utils.CertsDir(""))
			if err != nil {
				return fmt.Errorf("Unable to generate connector certificates: %s", err)
			}
		}

		// start the connector
		if !cc.IsRunning() {
			cb, err := cc.GetLocalCertBundle(utils.CertsDir(""))
			if err != nil {
				return fmt.Errorf("Unable to get certificates to secure ingress: %s", err)
			}

			l.Debug("Starting Ingress")

			err = cc.Start(cb)
			if err != nil {
				return fmt.Errorf("Unable to start ingress: %s", err)
			}
		}

		dst := ""
		if len(args) == 1 {
			dst = args[0]
		} else {
			dst = "./"
		}

		if dst == "." {
			dst = "./"
		}

		if dst != "" {
			cmd.Println("Running configuration from: ", dst)
			cmd.Println("")

			if !utils.IsLocalFolder(dst) && !utils.IsHCLFile(dst) {
				// fetch the remote server from github
				err := bp.Get(dst, utils.GetBlueprintLocalFolder(dst))
				if err != nil {
					return fmt.Errorf("Unable to retrieve blueprint: %s", err)
				}

				dst = utils.GetBlueprintLocalFolder(dst)
			}
		}

		// Parse the config to check it is valid
		err = e.ParseConfigWithVariables(dst, vars, *variablesFile)
		if err != nil {
			return fmt.Errorf("Unable to read config: %s", err)
		}

		// have we already got a blueprint in the state
		blueprintExists := false
		if bluePrintInState() {
			blueprintExists = true
		}

		// check that the current shipyard version can process this blueprint
		if e.Blueprint() != nil && e.Blueprint().ShipyardVersion != "" {
			valid, err := vm.InRange(version, e.Blueprint().ShipyardVersion)

			if !valid || err != nil {
				// we neeed to go in to the check loop
				return runWithOtherVersion(e.Blueprint().ShipyardVersion, *autoApprove, args, *force, *noOpen, cmd, vm, bc, *variables, *variablesFile)
			}
		}

		// update status every 30s to let people know we are still running
		statusUpdate := time.NewTicker(15 * time.Second)
		startTime := time.Now()

		go func() {
			for range statusUpdate.C {
				elapsedTime := time.Now().Sub(startTime).String()
				logger.Info("Please wait, still creating resources [Elapsed Time: " + elapsedTime + "]")
			}
		}()

		res, err := e.ApplyWithVariables(dst, vars, *variablesFile)
		if err != nil {
			return fmt.Errorf("Unable to apply blueprint: %s", err)
		}

		// do not open the browser windows
		if *noOpen == false {

			browserList := []string{}
			checkDuration := 30 * time.Second

			// check if blueprint is in the state, if so do not open these windows again
			if !blueprintExists && e.Blueprint() != nil {
				browserList = e.Blueprint().BrowserWindows
				// check for browser windows in the applied resources
				if e.Blueprint().HealthCheckTimeout != "" {
					cd, err := time.ParseDuration(e.Blueprint().HealthCheckTimeout)
					if err == nil {
						checkDuration = cd
					}
				}
			}

			for _, r := range res {
				switch r.Info().Type {
				case config.TypeContainer:
					c := r.(*config.Container)
					for _, p := range c.Ports {
						if p.Host != "" && p.OpenInBrowser != "" {
							browserList = append(browserList, buildBrowserPath(r.Info().Name, p.Host, r.Info().Type, p.OpenInBrowser))
						}
					}
				case config.TypeIngress:
					c := r.(*config.Ingress)
					if c.Source.Driver == config.IngressSourceLocal && c.Source.Config.OpenInBrowser != "" && c.Source.Config.Port != "" {
						browserList = append(browserList, buildBrowserPath(r.Info().Name, c.Source.Config.Port, r.Info().Type, c.Source.Config.OpenInBrowser))
					}
				case config.TypeContainerIngress:
					c := r.(*config.ContainerIngress)
					for _, p := range c.Ports {
						if p.Host != "" && p.OpenInBrowser != "" {
							browserList = append(browserList, buildBrowserPath(r.Info().Name, p.Host, r.Info().Type, p.OpenInBrowser))
						}
					}
				case config.TypeNomadIngress:
					c := r.(*config.NomadIngress)
					for _, p := range c.Ports {
						if p.Host != "" && p.OpenInBrowser != "" {
							browserList = append(browserList, buildBrowserPath(r.Info().Name, p.Host, r.Info().Type, p.OpenInBrowser))
						}
					}
				case config.TypeK8sIngress:
					c := r.(*config.K8sIngress)
					for _, p := range c.Ports {
						if p.Host != "" && p.OpenInBrowser != "" {
							browserList = append(browserList, buildBrowserPath(r.Info().Name, p.Host, r.Info().Type, p.OpenInBrowser))
						}
					}
				case config.TypeDocs:
					c := r.(*config.Docs)
					if c.OpenInBrowser {
						port := strconv.Itoa(c.Port)
						if port == "0" {
							port = "80"
						}

						browserList = append(browserList, buildBrowserPath(r.Info().Name, port, r.Info().Type, ""))
					}
				}
			}

			// check the browser windows in the blueprint file
			wg := sync.WaitGroup{}
			wg.Add(len(browserList))

			l.Debug("Health check urls for browser windows", "count", len(browserList))
			for _, b := range browserList {
				go func(uri string) {
					// health check the URL
					err := hc.HealthCheckHTTP(uri, []int{200}, checkDuration)
					if err == nil {
						be := bc.OpenBrowser(uri)
						if be != nil {
							l.Error("Unable to open browser", "error", be)
						}
					}

					wg.Done()
				}(b)
			}

			wg.Wait()
			l.Debug("Browser windows open")
		}

		// kill the timer
		statusUpdate.Stop()

		// if we have a blueprint show the header
		if e.Blueprint() != nil {
			cmd.Println("")
			cmd.Println("########################################################")
			cmd.Println("")
			cmd.Println("Title", e.Blueprint().Title)
			cmd.Println("Author", e.Blueprint().Author)

			// parse the body as markdown and print
			intro := markdown.Render(e.Blueprint().Intro, 80, 0)

			cmd.Println("")
			cmd.Print(string(intro))

			if len(e.Blueprint().Environment) > 0 || e.ResourceCountForType("output") > 0 {
				cmd.Println("")
				cmd.Printf("This blueprint defines %d output variables.\n", e.ResourceCountForType("output"))
				cmd.Println("")
				cmd.Println("You can set output variables as environment variables for your current terminal session using the following command:")
				cmd.Println("")

				if runtime.GOOS == "windows" {
					cmd.Println(`Invoke-Expression "shipyard env" | ForEach-Object { Invoke-Expression $_ }`)
				} else {
					cmd.Println("eval $(shipyard env)")
				}
				cmd.Println("")
				cmd.Println("To list output variables use the command:")
				cmd.Println("")
				cmd.Println("shipyard output")
			}
		}

		return nil
	}
}

func buildBrowserPath(n, p string, t config.ResourceType, path string) string {
	// if the path starts with http or https then override the default behaviour
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		// validate this is a good URL
		_, err := url.Parse(path)
		if err == nil {
			return path
		}
	}

	ty := t
	if t == config.TypeNomadIngress || t == config.TypeContainerIngress || t == config.TypeK8sIngress {
		ty = config.TypeIngress
	}

	return fmt.Sprintf("http://%s.%s.shipyard.run:%s%s", n, ty, p, path)
}

func bluePrintInState() bool {
	//load the state
	sc := config.New()
	sc.FromJSON(utils.StatePath())

	return sc.Blueprint != nil
}

func runWithOtherVersion(
	version string,
	autoApprove bool,
	args []string,
	forceUpdate bool,
	noBrowser bool,
	cmd *cobra.Command,
	vm gvm.Versions,
	sys clients.System,
	variables []string,
	variablesFile string) error {

	var exePath string

	r, err := vm.ListInstalledVersions(version)
	if err != nil {
		return err
	}

	// find a version suitable
	tag, url, err := vm.GetLatestReleaseURL(version)
	if err != nil {
		return err
	}

	if len(r) != 1 {

		// only prompt if not auto approve
		if !autoApprove {
			resp := sys.PromptInput(cmd.InOrStdin(), cmd.OutOrStdout(), fmt.Sprintf("Would you like to install version: %s [y/n]: ", tag))
			if resp != "y" {
				return nil
			}
		}

		exePath, err = vm.DownloadRelease(tag, url)
		if err != nil {
			return err
		}
	} else {
		exePath = r[tag]
	}

	// execute shipyard using a sub process
	cmd.Println("Running blueprint with version:", tag)

	//if there is no path use the current folder
	if len(args[0]) == 0 || args[0] == "" {
		p, _ := os.Getwd()
		args[0] = p
	} else if strings.HasPrefix(args[0], ".") {
		// if we have a relative path we need to convert to an absolute path
		p, _ := filepath.Abs(args[0])
		args[0] = p
	}

	commandString := []string{
		"run",
	}

	if forceUpdate {
		commandString = append(commandString, "--force-update")
	}

	if noBrowser {
		commandString = append(commandString, "--no-browser")
	}

	// Variables as a command line is only available in Shipyard v0.0.38 and later
	if ok, _ := vm.InRange(version, ">= v0.0.38"); ok {
		for _, v := range variables {
			commandString = append(commandString, "--var")
			commandString = append(commandString, v)
		}
	}

	if ok, _ := vm.InRange(version, ">= 0.1.6"); ok {
		if variablesFile != "" {
			commandString = append(commandString, "--vars-file="+variablesFile)
		}
	}

	commandString = append(commandString, args[0])

	execCmd := exec.Command(exePath, commandString...)
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdout = cmd.ErrOrStderr()

	err = execCmd.Start()
	if err != nil {
		return err
	}

	err = execCmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
