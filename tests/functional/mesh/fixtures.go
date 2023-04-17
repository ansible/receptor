package mesh

import "github.com/ansible/receptor/pkg/workceptor"

var workPlugins = []workPlugin{"command", "kube"}

var (
	workTestConfigs = map[workPlugin]map[workType]workceptor.WorkerConfig{
		"kube": {
			"echosleepshort": workceptor.KubeWorkerCfg{
				WorkType:   "echosleepshort",
				AuthMethod: "kubeconfig",
				Namespace:  "default",
				Image:      "alpine",
				Command:    "sh -c 'for i in `seq 1 5`; do echo $i;done'",
			},
			"echosleeplong": workceptor.KubeWorkerCfg{
				WorkType:   "echosleeplong",
				AuthMethod: "kubeconfig",
				Namespace:  "default",
				Image:      "alpine",
				Command:    "sh -c 'for i in `seq 1 5`; do echo $i; sleep 3;done'",
			},
			"echosleeplong50": workceptor.KubeWorkerCfg{
				WorkType:   "echosleeplong50",
				AuthMethod: "kubeconfig",
				Namespace:  "default",
				Image:      "alpine",
				Command:    "sh -c 'for i in `seq 1 50`; do echo $i; sleep 4;done'",
			},
		},
		"command": {
			"echosleepshort": workceptor.CommandWorkerCfg{
				WorkType: "echosleepshort",
				Command:  "bash",
				Params:   "-c 'for i in {1..5}; do echo $i;done'",
			},
			"echosleeplong": workceptor.CommandWorkerCfg{
				WorkType: "echosleeplong",
				Command:  "bash",
				Params:   "-c 'for i in {1..5}; do echo $i; sleep 3;done'",
			},
			"echosleeplong50": workceptor.CommandWorkerCfg{
				WorkType: "echosleeplong50",
				Command:  "base",
				Params:   "-c 'for i in {1..50}; do echo $i; sleep 4;done'",
			},
		},
	}
)
