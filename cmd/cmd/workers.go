package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// workersCmd represents the workers command
var workersCmd = &cobra.Command{
	Use:   "workers",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("workers called")
	},
}

func init() {
	rootCmd.AddCommand(workersCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// workersCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// workersCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// Commands to configure workers that process units of work:

//    --work-signing: Private key to sign work submissions
//       privatekey=<string>: Private key to sign work submissions
//       tokenexpiration=<string>: Expiration of the signed json web token, e.g. 3h or 3h30m

//    --work-verification: Public key to verify work submissions
//       publickey=<string>: Public key to verify signed work submissions

//    --work-command: Run a worker using an external command
//       worktype=<string>: Name for this worker type (required)
//       command=<string>: Command to run to process units of work (required)
//       params=<string>: Command-line parameters
//       allowruntimeparams=<bool>: Allow users to add more parameters (default: false)
//       verifysignature=<bool>: Verify a signed work submission (default: false)

//    --work-kubernetes: Run a worker using Kubernetes
//       worktype=<string>: Name for this worker type (required)
//       namespace=<string>: Kubernetes namespace to create pods in
//       image=<string>: Container image to use for the worker pod
//       command=<string>: Command to run in the container (overrides entrypoint)
//       params=<string>: Command-line parameters to pass to the entrypoint
//       authmethod=<string>: One of: kubeconfig, incluster (default: incluster)
//       kubeconfig=<string>: Kubeconfig filename (for authmethod=kubeconfig)
//       pod=<string>: Pod definition filename, in json or yaml format
//       allowruntimeauth=<bool>: Allow passing API parameters at runtime (default: false)
//       allowruntimecommand=<bool>: Allow specifying image & command at runtime (default: false)
//       allowruntimeparams=<bool>: Allow adding command parameters at runtime (default: false)
//       allowruntimepod=<bool>: Allow passing Pod at runtime (default: false)
//       deletepodonrestart=<bool>: On restart, delete the pod if in pending state (default: true)
//       streammethod=<string>: Method for connecting to worker pods: logger or tcp (default: logger)
//       verifysignature=<bool>: Verify a signed work submission (default: false)

//    --work-python: Run a worker using a Python plugin
//       worktype=<string>: Name for this worker type (required)
//       plugin=<string>: Python module name of the worker plugin (required)
//       function=<string>: Receptor-exported function to call (required)
//       config=<JSON dict with string keys>: Plugin-specific configuration
