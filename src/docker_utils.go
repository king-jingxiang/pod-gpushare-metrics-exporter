package main

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/golang/glog"
	"strings"
)

const dockerSock = "unix:///var/run/docker.sock"

// errorCode: 0: not running -1: unknown error
func getContainerPid(containerName string) int {
	glog.V(4).Infof("get container [%s] pid ", containerName)
	cli, err := client.NewClientWithOpts(client.WithHost(dockerSock), client.WithAPIVersionNegotiation())
	if err == nil {
		container, err := cli.ContainerInspect(context.Background(), containerName)
		if err == nil {
			return container.State.Pid
		} else {
			glog.V(4).Infof("docker pid find error %s", err)
		}
	} else {
		glog.V(4).Infof("docker pid find error %s", err)
	}
	return -1
}

func grepContainerPid(containerName string) int {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerSock), client.WithAPIVersionNegotiation())
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}
	for _, container := range containers {
		if strings.Contains(container.Names[0], containerName) {
			container, err := cli.ContainerInspect(context.Background(), container.ID)
			if err == nil {
				return container.State.Pid
			}
		}
	}
	return -1
}

func getMachineHostname() string {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerSock), client.WithAPIVersionNegotiation())
	info, err := cli.Info(context.Background())
	if err != nil {
		panic(err)
		return ""
	}

	return info.Name
}
