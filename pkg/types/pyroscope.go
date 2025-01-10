package types

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/grafana/pyroscope-go"
	"gopkg.in/yaml.v2"
)

type ReceptorPyroscopeCfg struct {
	ApplicationName   string // e.g backend.purchases
	Tags              map[string]string
	ServerAddress     string // e.g http://pyroscope.services.internal:4040
	BasicAuthUser     string // http basic auth user
	BasicAuthPassword string // http basic auth password
	TenantID          string // specify TenantId when using phlare multi-tenancy
	UploadRate        string
	ProfileTypes      []string
	DisableGCRuns     bool // this will disable automatic runtime.GC runs between getting the heap profiles
	HTTPHeaders       map[string]string
}

type UploadRate struct {
	UploadRate time.Duration `yaml:"uploadRate"`
}

func (pyroscopeCfg ReceptorPyroscopeCfg) Init() error {
	if pyroscopeCfg.ApplicationName == "" {
		return nil
	}
	_, err := pyroscope.Start(pyroscope.Config {
		ApplicationName: pyroscopeCfg.ApplicationName,
		Tags: pyroscopeCfg.Tags,
		ServerAddress: pyroscopeCfg.ServerAddress,
		BasicAuthUser: pyroscopeCfg.BasicAuthUser,
		BasicAuthPassword: pyroscopeCfg.BasicAuthPassword,
		TenantID: pyroscopeCfg.TenantID,
		UploadRate: getUploadRate(pyroscopeCfg),
		Logger: pyroscope.StandardLogger,
		ProfileTypes: getProfileTypes(pyroscopeCfg),
		DisableGCRuns: pyroscopeCfg.DisableGCRuns,
		HTTPHeaders: pyroscopeCfg.HTTPHeaders,
	})

	if err != nil {
		return err
	} else {
		startPyroscopeContainer(pyroscopeCfg)
		return nil
	}

	// pyroscope.TagWrapper(context.Background(), pyroscope.Labels("test", "netceptor_main"), func(c context.Context) {
	// 	netceptor.MainInstance = netceptor.New(context.Background(), cfg.ID)
	// })
}

func getUploadRate(cfg ReceptorPyroscopeCfg) time.Duration {
	if cfg.UploadRate == "" {
		return 15 * time.Second
	}
	var uploadRate UploadRate
	err := yaml.Unmarshal([]byte(cfg.UploadRate), &uploadRate)
	if err != nil {
		fmt.Println("failed to parse uploadRate from config file")
	}
	return uploadRate.UploadRate
}

func getProfileTypes(cfg ReceptorPyroscopeCfg) []pyroscope.ProfileType {
	profileType := []pyroscope.ProfileType{
		pyroscope.ProfileCPU,
		pyroscope.ProfileAllocObjects,
		pyroscope.ProfileAllocSpace,
		pyroscope.ProfileInuseObjects,
		pyroscope.ProfileInuseSpace,
	}
	if len(cfg.ProfileTypes) == 0 {
		return profileType
	}
	for _, pt := range cfg.ProfileTypes {
		switch pt {
		case "ProfileGoroutines":
			profileType = append(profileType, pyroscope.ProfileGoroutines)
		case "ProfileMutexCount":
			profileType = append(profileType, pyroscope.ProfileMutexCount)
		case "ProfileMutexDuration":
			profileType = append(profileType, pyroscope.ProfileMutexDuration)
		case "ProfileBlockCount":
			profileType = append(profileType, pyroscope.ProfileBlockCount)
		case "ProfileBlockDuration":
			profileType = append(profileType, pyroscope.ProfileBlockDuration)
		}
	}
	return profileType
}

func startPyroscopeContainer(cfg ReceptorPyroscopeCfg) {
	hostIp := strings.Split(strings.Split(cfg.ServerAddress, ":")[1], "//")[1]
	hostPort := strings.Split(cfg.ServerAddress, ":")[2]
	fmt.Println(hostIp)
	fmt.Println(hostPort)
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("failed to create docker client with error: %v", err)
	}
	defer cli.Close()

	imageName := "grafana/pyroscope"
	
	out, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		fmt.Printf("error trying to pull image with error: %v", err)
	}
	defer out.Close()
	io.Copy(os.Stdout, out)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		ExposedPorts: map[nat.Port]struct{}{
			"4040/tcp": {},
		},
	}, &container.HostConfig{
		PortBindings: map[nat.Port][]nat.PortBinding{
			"4040/tcp": {
				{HostIP: hostIp, HostPort: hostPort},
			},
		},
	}, nil, nil, "")
	if err != nil {
		fmt.Printf("error create container with error: %v", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		fmt.Printf("error starting container with error: %v", err)
	}
}
