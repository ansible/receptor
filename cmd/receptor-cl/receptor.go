package main

import (
	"os"

	_ "net/http/pprof"

	"github.com/ansible/receptor/cmd"
	"github.com/ansible/receptor/pkg/logger"
	"github.com/ansible/receptor/pkg/netceptor"
)

func main() {
	// pyroscope.Start(pyroscope.Config {
	// 	ApplicationName: "receptor",

	// 	// replace this with the address of pyroscope server
	// 	ServerAddress: "http://localhost:4040",

	// 	// you can disable logging by setting this to nil
	// 	Logger: pyroscope.StandardLogger,

	// 	ProfileTypes: [] pyroscope.ProfileType {
	// 		// these profile types are enabled by default:
	// 		pyroscope.ProfileCPU,
	// 			pyroscope.ProfileAllocObjects,
	// 			pyroscope.ProfileAllocSpace,
	// 			pyroscope.ProfileInuseObjects,
	// 			pyroscope.ProfileInuseSpace,

	// 			// these profile types are optional:
	// 			pyroscope.ProfileGoroutines,
	// 			pyroscope.ProfileMutexCount,
	// 			pyroscope.ProfileMutexDuration,
	// 			pyroscope.ProfileBlockCount,
	// 			pyroscope.ProfileBlockDuration,
	// 	},
	// })

	// pyroscope.TagWrapper(context.Background(), pyroscope.Labels("test", "netceptor_main"), func(c context.Context) {
	// 	netceptor.MainInstance = netceptor.New(context.Background(), cfg.ID)
	// })

	

	logger := logger.NewReceptorLogger("")
	var isV2 bool
	newArgs := []string{}
	for _, arg := range os.Args {
		if arg == "--config-v2" {
			isV2 = true

			continue
		}
		newArgs = append(newArgs, arg)
	}

	os.Args = newArgs

	if isV2 {
		logger.Info("Running v2 cli/config")
		cmd.Execute()
	} else {
		cmd.RunConfigV1()
	}

	for _, arg := range os.Args {
		if arg == "--help" || arg == "-h" {
			os.Exit(0)
		}
	}

	if netceptor.MainInstance.BackendCount() == 0 {
		logger.Warning("Nothing to do - no backends are running.\n")
		logger.Warning("Run %s --help for command line instructions.\n", os.Args[0])
		os.Exit(1)
	}

	logger.Info("Initialization complete\n")

	<-netceptor.MainInstance.NetceptorDone()
}
