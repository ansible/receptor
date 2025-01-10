package utils

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func StartPyroscopeContainer(hostIP string, hostPort string) {
	ctx := context.Background()
	defer ctx.Done()
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
				{HostIP: hostIP, HostPort: hostPort},
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
