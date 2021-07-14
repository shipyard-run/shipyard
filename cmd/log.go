package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TwinProduction/go-color"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"

	"github.com/shipyard-run/shipyard/pkg/clients"
	"github.com/shipyard-run/shipyard/pkg/config"
	"github.com/shipyard-run/shipyard/pkg/shipyard"
	"github.com/shipyard-run/shipyard/pkg/utils"
)

// print debug
const debug = false

// this is for auto-complete only which is not yet implemented
const cKey = "container"
const k8Key = "k8s_cluster"
const format = "%-8s\t%s"

// bpLogs defines the options for obtaining logs for a shipyard blueprint that is currently running
type bpLogs struct {
	// output to redirect the logs
	commonOut *os.File
	// cancelling on user interrupt
	closeLogs chan os.Signal
	ctx       context.Context
	cancel    context.CancelFunc
	// print cli utility debug
	debugOn bool
	// follow stdout, stdErr options for docker container
	dockerLogsOpts types.ContainerLogsOptions
	// shipyard engine
	engineClients *shipyard.Clients
	// color
	color colorI

	// cl map[string]types.Container // containers
	// pl map[string]v1.Pod // pods

	// from blueprint
	stack stackI // type<->container/clusterName
	// after client connection
	connections stackI
}

func logCmd(out *os.File, engine shipyard.Engine) *cobra.Command {
	return &cobra.Command{
		Use:   "log <command> ",
		Short: "Tails logs for all containers/clusters of the currently active blueprint (wip)",
		Long:  `Tails logs for all containers/clusters of the currently active blueprint (wip)`,
		Example: `
# List containers & clusters from the current shipyard blueprint
	shipyard log

# Tail logs for all shipyard containers
	shipyard log containers

# Tail logs for a kubernetes clusters
	shipyard log cluster $cluster-name
	`,
		Args: cobra.ArbitraryArgs,
		/*
			 ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {

				bp := newStack(parseStack())

				//fmt.Println(bp.toNames(typeBpTypes, ""))
				//fmt.Println(bp.toNames(typeBpContainers, cKey))
				//fmt.Println(bp.toNames(typeBpClusters, k8Key))

				//slog := InitSLogs(out)
				//slog.engineClients = engine.GetClients()
				//connectKube(slog, slog.engineClients.Kubernetes)
				//fetchAllContainers(slog)

				//slog.PrintToOutput(fmt.Sprintf(format, cKey, slog.stack.toNames(typeConnContainers, cKey)))
				//slog.PrintToOutput(fmt.Sprintf(format, "pods -", slog.stack.toNames(typeConnPods, "todo_not_used")))

				return bp.toNames(typeBpTypes, ""), cobra.ShellCompDirectiveDefault
			},
		*/

		Run: func(cmd *cobra.Command, args []string) {

			slog := InitSLogs(out)

			// execInput and start logging to StdOut
			if len(args) == 0 {
				// print stack
				slog.stack.printStack(slog.commonOut)
				// slog.PrintToOutput("")
				// slog.PrintToOutput(slog.stack.toNames(typeBpTypes, ""))
				// slog.PrintToOutput(fmt.Sprintf(format, cKey, slog.stack.toNames(typeBpContainers, cKey)))
				// slog.PrintToOutput(fmt.Sprintf(format, k8Key, slog.stack.toNames(typeBpClusters, k8Key)))

				// `go test` throws error if os.exit() is called during a test case.
				// This check is only to ensure os.exit() is called only in non-test case
				if slog.commonOut == os.Stdout {
					os.Exit(0)
				} else {
					return
				}
			}
			slog.engineClients = engine.GetClients()

			// check args and print logs
			if slog.execInput(args) {
				// wait till user interrupt
				<-slog.closeLogs
			}
			// cancel streams and exit
			slog.cancel()
			if debug {
				time.Sleep(500 * time.Millisecond) // ensures all debug statements gets printed before exit
			}
		},
	}
}

// InitSLogs creates a bpLogs object and parses the current shipyard stack
func InitSLogs(out *os.File) *bpLogs {
	// Routine to catch user interrupt to stop tailing the logs
	closeLogs, ctx, cancel := catchInterrupt()
	slog := &bpLogs{
		commonOut: out,
		closeLogs: closeLogs,
		ctx:       ctx,
		cancel:    cancel,
		debugOn:   debug,
		color:     newColor(),
		dockerLogsOpts: types.ContainerLogsOptions{
			ShowStdout: true, // always true
			ShowStderr: true, // can be false
			Follow:     true, // always true, can stop with ctrl+c
		},
		//pl: make(map[string]v1.Pod),
		//cl: make(map[string]types.Container),

		// for auto-complete
		connections: newStack(nil),          // connections
		stack:       newStack(parseStack()), // blueprint
	}
	return slog
}

// execInput parse cli inputs and starts streaming the logs.
// todo -> flags?
func (slog *bpLogs) execInput(args []string) bool {

	switch args[0] {
	case "containers":
		if !slog.allContainerLogs() {
			slog.PrintToOutput("Could not get docker containers")
			return false
		}
		return true
	case "cluster":
		if len(args) == 2{
			if connectKube(slog, slog.engineClients.Kubernetes, args[1]) {
				if !slog.kubernetesLogs() {
					slog.PrintToOutput("Could not load connect to pods")
					return false
				}
				return true
			} else {
				slog.PrintToOutput("Could not load KubeConfig")
				return false
			}
		}else {
			slog.PrintToOutput("Cluster not found")
			return false
		}
	default:
		slog.PrintToOutput("Bad args")
		return false
	}
}

// PrintToOutput prints to configured *os.File
func (slog *bpLogs) PrintToOutput(i ...interface{}) {
	_, _ = fmt.Fprintln(slog.commonOut, i...)
}

// catchInterrupt sets up a os signal catch routine to stop all streams
// and exit the shipyard cli utility. It return a os.signal channel and
// contextWithCancel
func catchInterrupt() (chan os.Signal, context.Context, context.CancelFunc) {
	closeLogs := make(chan os.Signal, 1)
	signal.Notify(closeLogs, os.Interrupt, syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGTERM, syscall.SIGQUIT)
	// to cancel engine.client log streams
	ctx, cancel := context.WithCancel(context.Background())
	return closeLogs, ctx, cancel
}
func printDebug(i ...interface{}) {
	if debug {
		fmt.Println(i...)
	}
}

// parse the stack and save resources to a map
func parseStack() map[string][]string {
	c := config.New()
	err := c.FromJSON(utils.StatePath())
	if err != nil {
		fmt.Println("Unable to load state", err)
		os.Exit(1)
	}
	stack := make(map[string][]string)
	for _, r := range c.Resources {
		 if string(r.Info().Type) == "k8s_cluster" || string(r.Info().Type) == "container" && r.Info().Status == "applied" { // not sure
			stack[string(r.Info().Type)] = append(stack[string(r.Info().Type)], r.Info().Name)
		}
		// }
	}
	return stack
}

func (slog *bpLogs) allContainerLogs() bool {
	defer slog.ctx.Done()
	containers, done := fetchAllContainers(slog)
	if !done {
		return false
	}
	c := slog.engineClients.Docker
	logging := false
	for _, container := range containers {
		if strings.Contains(container.Names[0], "docker-cache") ||
			strings.Contains(container.Names[0], "k8s_cluster") ||
			strings.Contains(container.Names[0], "ingress.shipyard.run") {
			continue
		}
		if logReader, err := c.ContainerLogs(slog.ctx, container.ID, slog.dockerLogsOpts); err == nil {
			// colorize container's prefix
			sOutPrefix := color.Ize(slog.color.nextColor(), container.Names[0][1:])
			if slog.dockerLogsOpts.ShowStderr {
				// logReader has both stdout and stderr, de-mux them
				go slog.splitAndAddReaders(sOutPrefix, logReader)
			} else {
				slog.startLogging(sOutPrefix, logReader)
			}
			logging = true
		}
	}
	return logging
}

func (slog *bpLogs) kubernetesLogs() bool {
	pl, ok := slog.getPods()
	if !ok {
		return false
	}
	for _, pod := range pl.Items {
		if "Running" == string(pod.Status.Phase) &&
			strings.Contains(pod.Namespace, "default") { // todo replace strings.contains with label selector?
			go func(pod v1.Pod) {
				if logReader, err := slog.engineClients.Kubernetes.GetPodLogs(slog.ctx,
					pod.Name, pod.Namespace); err == nil {
					sOutPrefix := color.Ize(slog.color.nextColor(), pod.Name)
					slog.startLogging(sOutPrefix, logReader)
				}
			}(pod)
		}
	}
	return true
}

// connectKube connects if $KubeConfig is set
func connectKube(slog *bpLogs, cli clients.Kubernetes, clusterName string) bool {
	var err error
	kubeCfg := fmt.Sprintf("%s/config/%s/kubeconfig.yaml",utils.ShipyardHome(), clusterName)
//	if _, err = os.Stat(os.Getenv("KUBECONFIG")); err != nil {
	if _, err = os.Stat(kubeCfg); err != nil {
		printDebug(err.Error())
		return false
	}
	// slog.engineClients.Kubernetes, err = cli.SetConfig(os.Getenv("KUBECONFIG"))
	slog.engineClients.Kubernetes, err = cli.SetConfig(kubeCfg)
	
	if err != nil {
		printDebug(err.Error())
		return false
	}
	// return all for now
	return fetchAllPods(slog)
}

func fetchAllPods(slog *bpLogs) bool {
	pl, ok := slog.getPods()
	defaultPods := new([]v1.Pod)
	if ok {
		for _, pod := range pl.Items {
			if strings.Contains(pod.Namespace, "default") { // todo
				// slog.pl[pod.Name] = pod
				*defaultPods = append(*defaultPods, pod)
			}
		}
	}
	slog.stack.addConn("todo_not_used", *defaultPods) // []v1.Pod
	return ok
}
func (slog *bpLogs) getPods() (*v1.PodList, bool) {
	pl, err := slog.engineClients.Kubernetes.GetPods("")
	if err != nil {
		printDebug(err.Error())
		return nil, false
	}
	return pl, true
}

// fetchAllContainers returns a list of docker containers along with a flag to indicate any error
func fetchAllContainers(slog *bpLogs) ([]types.Container, bool) {
	filter := filters.NewArgs() // equivalent for k8 ?
	filter.Add("name", "shipyard")
	filter.Add("status", "running")
	containers, err := slog.engineClients.Docker.ContainerList(slog.ctx, types.ContainerListOptions{
		Filters: filter,
	})
	if err != nil || len(containers) == 0 {
		return nil, false
	}
	// for _, c := range containers {
	// slog.cl[c.Names[0][1:]] = c
	// }
	slog.stack.addConn(cKey, containers) // []types.Container
	return containers, true
}

// splitAndAddReaders splits a io.ReadCloser that is multiplexed as a common StdOut + StdErr
// into two separate io.ReadCloser. It then calls startLogging for both streams
func (slog *bpLogs) splitAndAddReaders(prefix string, logReader io.ReadCloser) {
	// Create io pipes to split logReader to StdOut and StdErr
	stdOutReadr, dstOut := io.Pipe()
	stdErrReadr, dstErr := io.Pipe()

	// close io pipes when ctx is cancelled
	go waitClose(prefix, slog.ctx, logReader, dstOut, dstErr, stdOutReadr, stdErrReadr)

	// de-multiplex logReader to stdout and stderr streams
	go deMuxStream(prefix, dstOut, dstErr, logReader)

	slog.startLogging(prefix, stdOutReadr)
	slog.startLogging(prefix, stdErrReadr)
}

// startLogging reads from the io.ReadCloser, adds the prefix and prints to the common out
func (slog *bpLogs) startLogging(prefix string, logReader io.ReadCloser) {
	go func() {
		defer func(logReader io.ReadCloser) {
			_ = logReader.Close()
		}(logReader)
		scanner := bufio.NewScanner(logReader)
		printDebug("Added reader for", prefix)
		for {
			select {
			case <-slog.ctx.Done():
				printDebug("stopped reader for", prefix)
				return
			default:
				for scanner.Scan() {
					log := "[" + prefix + "] " + scanner.Text()
					_, err := fmt.Fprintln(slog.commonOut, log)
					if err != nil {
						fmt.Println(err.Error())
					}
				}
			}
		}
	}()
}

// deMuxStream splits readCloser into StdErr and StdOut if readCloser was written
// to in this way
func deMuxStream(name string, dstOut *io.PipeWriter, dstErr *io.PipeWriter, logReader io.ReadCloser) {
	_, _ = stdcopy.StdCopy(dstOut, dstErr, logReader)
	printDebug("Stopped de-mux-ing log streams for ", name)
}

// waitClose waits for the context to finish then closes all reader and writers
func waitClose(name string, ctx context.Context, logReader io.ReadCloser, dstOut *io.PipeWriter,
	dstErr *io.PipeWriter, stdOutReadr *io.PipeReader, stdErrReadr *io.PipeReader) {
	<-ctx.Done()
	_ = logReader.Close()
	_ = dstOut.Close()
	_ = dstErr.Close()
	_ = stdOutReadr.Close()
	_ = stdErrReadr.Close()
	printDebug("Stopped reading log streams for ", name)
}
