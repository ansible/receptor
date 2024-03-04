package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	workSigningPrivateKey             string
	workSigningTokenExpiration        string
	workVerificationPublicKey         string
	workCommandWorkType               string
	workCommandCommand                string
	workCommandParams                 string
	workCommandAllowRuntimeParams     bool
	workCommandVerifySignature        bool
	workKubernetesWorkType            string
	workKubernetesNamespace           string
	workKubernetesImage               string
	workKubernetesCommand             string
	workKubernetesParams              string
	workKubernetesAuthMethod          string
	workKubernetesKubeConfig          string
	workKubernetesPod                 string
	workKubernetesAllowRuntimeAuth    bool
	workKubernetesAllowRuntimeCommand bool
	workKubernetesAllowRuntimeParams  bool
	workKubernetesAllowRuntimePod     bool
	workKubernetesDeletePodOnRestart  bool
	workKubernetesStreamMethod        string
	workKubernetesVerifySignature     bool
	workPythonWorkType                string
	workPythonPlugin                  string
	workPythonFunction                string
	workPythonConfig                  map[string]string
)

var workSigning = &cobra.Command{
	Use:   "work-signing",
	Short: "Private key to sign work submissions",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var workVerification = &cobra.Command{
	Use:   "work-verification",
	Short: "Public key to verify work submissions",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var workCommand = &cobra.Command{
	Use:   "work-command",
	Short: "Run a worker using an external command",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var workKubernetes = &cobra.Command{
	Use:   "work-kubernetes",
	Short: "Run a worker using Kubernetes",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

var workPython = &cobra.Command{
	Use:   "work-python",
	Short: "Run a worker using a Python plugin",
	Long:  ``,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("resources called")
	},
}

func init() {
	workSigning.Flags().StringVar(&workSigningPrivateKey, "privatekey", "", "Private key to sign work submissions")
	workSigning.MarkFlagRequired("privatekey")
	workSigning.Flags().StringVar(&workSigningTokenExpiration, "tokenexpiration", "", "Expiration of the signed json web token, e.g. 3h or 3h30m")
	workSigning.MarkFlagRequired("tokenexpiration")

	workVerification.Flags().StringVar(&workVerificationPublicKey, "publickey", "", "Public key to verify signed work submissions")
	workVerification.MarkFlagRequired("publickey")

	workCommand.Flags().StringVar(&workCommandWorkType, "worktype", "", "Name for this worker type (required)")
	workCommand.MarkFlagRequired("worktype")
	workCommand.Flags().StringVar(&workCommandCommand, "command", "", "Command to run to process units of work (required)")
	workCommand.MarkFlagRequired("command")
	workCommand.Flags().StringVar(&workCommandParams, "params", "", "Command-line parameters")
	workCommand.Flags().BoolVar(&workCommandAllowRuntimeParams, "allowruntimeparams", false, "Allow users to add more parameters (default: false)")
	workCommand.Flags().BoolVar(&workCommandVerifySignature, "verifysignature", false, "Verify a signed work submission (default: false)")

	workKubernetes.Flags().StringVar(&workKubernetesWorkType, "worktype", "", "Name for this worker type (required)")
	workKubernetes.MarkFlagRequired("worktype")
	workKubernetes.Flags().StringVar(&workKubernetesNamespace, "namespace", "", "Kubernetes namespace to create pods in")
	workKubernetes.Flags().StringVar(&workKubernetesImage, "image", "", "Container image to use for the worker pod")
	workKubernetes.Flags().StringVar(&workKubernetesCommand, "command", "", "Command to run in the container (overrides entrypoint)")
	workKubernetes.Flags().StringVar(&workKubernetesParams, "params", "", "Command-line parameters to pass to the entrypoint")
	workKubernetes.Flags().StringVar(&workKubernetesAuthMethod, "authmethod", "incluster", "One of: kubeconfig, incluster (default: incluster)")
	workKubernetes.Flags().StringVar(&workKubernetesKubeConfig, "kubeconfig", "", "Kubeconfig filename (for authmethod=kubeconfig)")
	workKubernetes.Flags().StringVar(&workKubernetesPod, "pod", "", "Pod definition filename, in json or yaml format")
	workKubernetes.Flags().BoolVar(&workKubernetesAllowRuntimeAuth, "allowruntimeauth", false, "Allow passing API parameters at runtime (default: false)")
	workKubernetes.Flags().BoolVar(&workKubernetesAllowRuntimeCommand, "allowruntimecommand", false, "Allow specifying image & command at runtime (default: false)")
	workKubernetes.Flags().BoolVar(&workKubernetesAllowRuntimeParams, "allowruntimeparams", false, "Allow adding command parameters at runtime (default: false)")
	workKubernetes.Flags().BoolVar(&workKubernetesAllowRuntimePod, "allowruntimepod", false, "Allow passing Pod at runtime (default: false)")
	workKubernetes.Flags().BoolVar(&workKubernetesDeletePodOnRestart, "deletepodonrestart", true, "On restart, delete the pod if in pending state (default: true)")
	workKubernetes.Flags().StringVar(&workKubernetesStreamMethod, "streammethod", "logger", "Method for connecting to worker pods: logger or tcp (default: logger)")
	workKubernetes.Flags().BoolVar(&workKubernetesVerifySignature, "verifysignature", false, "Verify a signed work submission (default: false)")

	workPython.Flags().StringVar(&workPythonWorkType, "worktype", "", "Name for this worker type (required)")
	workPython.MarkFlagRequired("worktype")
	workPython.Flags().StringVar(&workPythonPlugin, "plugin", "", "Python module name of the worker plugin (required)")
	workPython.MarkFlagRequired("plugin")
	workPython.Flags().StringVar(&workPythonFunction, "function", "", "Receptor-exported function to call (required)")
	workPython.MarkFlagRequired("function")
	workPython.Flags().StringToStringVarP(&workPythonConfig, "config", "", nil, "Plugin-specific configuration")

	rootCmd.AddCommand(workSigning, workVerification, workCommand, workKubernetes, workPython)

}
