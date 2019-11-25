// Copyright (c) 2018, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"bufio"
	"fmt"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/golang/glog"
	"github.com/mitchellh/go-ps"
	"io"
	"io/ioutil"
	"os"
	"strings"

	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"
)

const nvidiaResourceName = "nvidia.com/gpu"

// 存储map[uuid]index
var gpuUUID = make(map[string]uint)

type devicePodInfo struct {
	name      string
	namespace string
	container string
}

// Helper function that creates a map of pod info for each device
func createDevicePodMap(devicePods podresourcesapi.ListPodResourcesResponse) map[string]devicePodInfo {
	deviceToPodMap := make(map[string]devicePodInfo)

	for _, pod := range devicePods.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == nvidiaResourceName {
					podInfo := devicePodInfo{
						name:      pod.GetName(),
						namespace: pod.GetNamespace(),
						container: container.GetName(),
					}
					for _, uuid := range device.GetDeviceIds() {
						deviceToPodMap[uuid] = podInfo
					}
				}
			}
		}
	}
	return deviceToPodMap
}

// todo add createProcessContainerMap
// todo add createContainerPodMap
// todo 最终目的 add createProcessPodMap
type processPodInfo struct {
	id            int
	uuid          string
	name          string
	namespace     string
	container     string
	processName   string
	processPid    uint
	processType   string
	processMemory uint64
}

// Helper function that creates a map of pod info for each process
func createProcessPodMap(devicePods podresourcesapi.ListPodResourcesResponse) map[string]processPodInfo {
	processToPodMap := make(map[string]processPodInfo)

	for _, pod := range devicePods.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == nvidiaResourceName {
					for _, uuid := range device.GetDeviceIds() {
						id := getGPUIdByUUID(uuid)
						d, _ := nvml.NewDeviceLite(id)
						if d.UUID == uuid {
							infos, _ := d.GetAllRunningProcesses()
							for _, v := range infos {
								if checkProcessParent(fmt.Sprintf("k8s_%s_%s_%s", container.Name, pod.Name, pod.Namespace), v.PID) {
									podInfo := processPodInfo{
										id:            int(id),
										uuid:          uuid,
										name:          pod.GetName(),
										namespace:     pod.GetNamespace(),
										container:     container.GetName(),
										processName:   v.Name,
										processPid:    v.PID,
										processType:   v.Type.String(),
										processMemory: v.MemoryUsed,
									}
									processToPodMap[fmt.Sprintf("%s-%s-%s", id, uuid, v.PID)] = podInfo
								}
							}
						}
					}
				}
			}
		}
	}
	return processToPodMap
}

// 通过检查进程container process是否是gpuprocess的父进程判断是否是容器内进程
func checkProcessParent(containerName string, gPid uint) bool {
	cPid := grepContainerPid(containerName)
	gProcess, err := ps.FindProcess(int(gPid))
	if err != nil {
		glog.V(4).Infof("find process error %v", err)
	}
	for {
		pprocess, err := ps.FindProcess(gProcess.PPid())
		if err != nil {
			glog.Errorf("find parent process filed: %v", err)
			return false
		}
		if pprocess.Pid() < 1000 {
			glog.Errorf("gpu process [%v] not found docker container parent process ", gPid)
			return false
		} else if pprocess.Pid() == cPid {
			glog.V(4).Infof("gpu process [%v] found docker container parent process [%v] ", gPid, cPid)
			return true
		}
		gProcess = pprocess
	}
}

// return uuid index
func getGPUIdByUUID(uuid string) uint {
	if _, found := gpuUUID[uuid]; !found {
		n, _ := nvml.GetDeviceCount()
		for i := uint(0); i < n; i++ {
			d, _ := nvml.NewDeviceLite(i)
			gpuUUID[d.UUID] = i
		}
	}
	return gpuUUID[uuid]
}

func getDevicePodInfo(socket string) (map[string]devicePodInfo, error) {
	devicePods, err := getListOfPods(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices Pod information: %v", err)
	}
	return createDevicePodMap(*devicePods), nil

}

func getProcessPodInfo(socket string) (map[string]processPodInfo, error) {
	devicePods, err := getListOfPods(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices Pod information: %v", err)
	}
	return createProcessPodMap(*devicePods), nil
}

func addPodInfoToMetrics(dir string, srcFile string, destFile string, deviceToPodMap map[string]devicePodInfo) error {
	readFI, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", srcFile, err)
	}
	defer readFI.Close()
	reader := bufio.NewReader(readFI)

	tmpPrefix := "pod"
	tmpF, err := ioutil.TempFile(dir, tmpPrefix)
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}

	tmpFname := tmpF.Name()
	defer func() {
		tmpF.Close()
		os.Remove(tmpFname)
	}()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(line) == 0 {
				return writeDestFile(tmpFname, destFile)
			}
			return fmt.Errorf("error reading %s: %v", srcFile, err)
		}

		// Skip comments and add pod info
		if string(line[0]) != "#" {
			uuid := strings.Split(strings.Split(line, ",")[1], "\"")[1]
			if pod, exists := deviceToPodMap[uuid]; exists {
				splitLine := strings.Split(line, "}")
				line = fmt.Sprintf("%s,pod_name=\"%s\",pod_namespace=\"%s\",container_name=\"%s\"}%s", splitLine[0], pod.name, pod.namespace, pod.container, splitLine[1])
			}
		}

		_, err = tmpF.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing to %s: %v", tmpFname, err)
		}
	}
}

// todo 输出到process文件
func addProcessInfoToMetrics(dir string, destFile string, processToPodMap map[string]processPodInfo) error {

	tmpPrefix := "process"
	tmpF, err := ioutil.TempFile(dir, tmpPrefix)
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}
	tmpFname := tmpF.Name()
	defer func() {
		tmpF.Close()
		os.Remove(tmpFname)
	}()
	_, err = tmpF.WriteString("# TYPE dcgm_process_mem_used gauge\n")
	if err != nil {
		return fmt.Errorf("error writing to %s: %v", tmpFname, err)
	}
	//# TYPE dcgm_process_mem_used gauge
	//dcgm_process_mem_used{gpu="0",uuid="GPU-ad365448-e6c2-68f2-24e4-517b1e56e937",pod_name="test-pod-01",pod_namespace="default",container_name="nvidia-test",process_name="python",process_pid="4000",process_type="computer"} 1024
	for _, pod := range processToPodMap {
		line := fmt.Sprintf("dcgm_process_mem_used{gpu=\"%v\",uuid=\"%s\",pod_name=\"%s\",pod_namespace=\"%s\",container_name=\"%s\",process_name=\"%s\",process_pid=\"%v\",process_type=\"%s\"}%v\n",
			pod.id, pod.uuid, pod.name, pod.namespace, pod.container, pod.processName, pod.processPid, pod.processType, pod.processMemory)
		_, err = tmpF.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing to %s: %v", tmpFname, err)
		}
	}
	return writeDestFile(tmpFname, destFile)
}
