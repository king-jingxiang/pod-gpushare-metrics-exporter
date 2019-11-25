package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
)

const dockerSock = "unix:///var/run/docker.sock"

func main() {
	fmt.Println(getContainerPid("cuda-100"))
}


// errorCode: 0: not running -1: unknown error
func getContainerPid(containerName string) int {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerSock))
	if err == nil {
		container, err := cli.ContainerInspect(context.Background(), containerName)
		if err == nil {
			return container.State.Pid
		}
	}
	return -1
}
