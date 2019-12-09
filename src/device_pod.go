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
var gpuUUIDMap = make(map[string]uint)

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
						trueUUID := GetTrueID(uuid)
						id := getGPUIdByUUID(trueUUID)
						d, _ := nvml.NewDeviceLite(id)
						if d.UUID == trueUUID {
							infos, _ := d.GetAllRunningProcesses()
							for _, v := range infos {
								if checkProcessParent(fmt.Sprintf("k8s_%s_%s_%s", container.Name, pod.Name, pod.Namespace), v.PID) {
									podInfo := processPodInfo{
										id:            int(id),
										uuid:          trueUUID,
										name:          pod.GetName(),
										namespace:     pod.GetNamespace(),
										container:     container.GetName(),
										processName:   v.Name,
										processPid:    v.PID,
										processType:   v.Type.String(),
										processMemory: v.MemoryUsed,
									}
									processToPodMap[fmt.Sprintf("%s-%s-%s", id, trueUUID, v.PID)] = podInfo
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

type gpuUsedInfo struct {
	hostname string
	id       uint
	uuid     string
	used     uint
}

// 获取gpu卡占用情况
func getGpuBasicInfo(devicePods podresourcesapi.ListPodResourcesResponse) map[string]gpuUsedInfo {
	// 逻辑分配
	gpuLogicUsedMap := make(map[string]gpuUsedInfo)
	// 物理占用
	//gpuPhysicalUsedMap := make(map[string]gpuUsedInfo)
	if len(gpuUUIDMap) <= 0 {
		initUUIDMap()
	}
	for uuid, id := range gpuUUIDMap {
		info := gpuUsedInfo{
			id:   id,
			uuid: uuid,
			used: 0,
		}
		gpuLogicUsedMap[uuid] = info
	}
	for _, pod := range devicePods.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == nvidiaResourceName {
					for _, uuid := range device.GetDeviceIds() {
						trueUUID := GetTrueID(uuid)
						if ginfo, found := gpuLogicUsedMap[trueUUID]; found {
							ginfo.used = 1
							gpuLogicUsedMap[trueUUID] = ginfo
						}
					}
				}
			}
		}
	}
	return gpuLogicUsedMap
}

// GetTrueID takes device name in k8s and return the true DeviceID in node
func GetTrueID(vid string) string {
	// todo for gpushare get ture uuid
	//trueUUID := strings.Split(uuid, "_")[0]
	if vid[len(vid)-2:len(vid)-1] != "-" {
		return vid
	}
	return vid[:len(vid)-2]
}

// 通过检查进程container process是否是gpuprocess的父进程判断是否是容器内进程
func checkProcessParent(containerName string, gPid uint) bool {
	cPid := grepContainerPid(containerName)
	cProcess, err := ps.FindProcess(cPid)
	if err != nil {
		glog.V(4).Infof("find container process error %v", err)
		return false
	}
	cInitPid := cProcess.PPid()
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
		if pprocess.Pid() <= 1 {
			glog.Errorf("gpu process [%v] not found docker container parent process ", gPid)
			return false
		} else if pprocess.Pid() == cInitPid {
			glog.V(4).Infof("gpu process [%v] found docker container parent process [%v] ", gPid, cInitPid)
			return true
		}
		gProcess = pprocess
	}
}

func initUUIDMap() {
	n, _ := nvml.GetDeviceCount()
	for i := uint(0); i < n; i++ {
		d, _ := nvml.NewDeviceLite(i)
		gpuUUIDMap[d.UUID] = i
	}
}

// return uuid index
func getGPUIdByUUID(uuid string) uint {
	if _, found := gpuUUIDMap[uuid]; !found {
		initUUIDMap()
	}
	return gpuUUIDMap[uuid]
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

func getGpuUsedInfo(socket string) (map[string]gpuUsedInfo, error) {
	devicePods, err := getListOfPods(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices Pod information: %v", err)
	}
	return getGpuBasicInfo(*devicePods), nil
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
	_, err = tmpF.WriteString("# HELP dcgm_process_mem_used process memory used (in MiB).\n")
	if err != nil {
		return fmt.Errorf("error writing to %s: %v", tmpFname, err)
	}
	//# TYPE dcgm_process_mem_used gauge
	//dcgm_process_mem_used{gpu="0",uuid="GPU-ad365448-e6c2-68f2-24e4-517b1e56e937",pod_name="test-pod-01",pod_namespace="default",container_name="nvidia-test",process_name="python",process_pid="4000",process_type="computer"} 1024
	for _, pod := range processToPodMap {
		line := fmt.Sprintf("dcgm_process_mem_used{gpu=\"%v\",uuid=\"%s\",pod_name=\"%s\",pod_namespace=\"%s\",container_name=\"%s\",process_name=\"%s\",process_pid=\"%v\",process_type=\"%s\"} %v\n",
			pod.id, pod.uuid, pod.name, pod.namespace, pod.container, pod.processName, pod.processPid, pod.processType, pod.processMemory)
		_, err = tmpF.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing to %s: %v", tmpFname, err)
		}
	}
	return writeDestFile(tmpFname, destFile)
}
func addGpuInfoInfoToMetrics(dir string, destFile string, gpuUsedMap map[string]gpuUsedInfo) error {

	tmpPrefix := "basic"
	tmpF, err := ioutil.TempFile(dir, tmpPrefix)
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}
	tmpFname := tmpF.Name()
	defer func() {
		tmpF.Close()
		os.Remove(tmpFname)
	}()
	_, err = tmpF.WriteString("# TYPE dcgm_gpu_logic_used gauge\n")
	_, err = tmpF.WriteString("# HELP dcgm_gpu_logic_used gpu used (in 0(unused)/1(used) ).\n")
	if err != nil {
		return fmt.Errorf("error writing to %s: %v", tmpFname, err)
	}
	// hostname 通过docker info获取
	hostname := getMachineHostname()
	var logicUsed [32]int
	for _, gpu := range gpuUsedMap {
		if gpu.used == 1 {
			logicUsed[int(gpu.id)] = 1
		}
	}
	logicUsedStr := strings.Trim(fmt.Sprint(logicUsed[:len(gpuUsedMap)]), "[]")
	//# HELP dcgm_gpu_logic_used gpu used (in 0(unused)/1(used) ).
	//dcgm_gpu_logic_used{hostname="pod-gpu-metrics-exporter-mvqs8",count="1",used="1"} 1
	line := fmt.Sprintf("dcgm_gpu_logic_used{hostname=\"%s\",count=\"%v\",used=\"%s\"} %v\n", hostname, len(gpuUsedMap), logicUsedStr, calcDec(logicUsed[:len(gpuUsedMap)]))
	_, err = tmpF.WriteString(line)
	if err != nil {
		return fmt.Errorf("error writing to %s: %v", tmpFname, err)
	}
	return writeDestFile(tmpFname, destFile)
}

func calcDec(data []int) int {
	value := 0
	len := len(data) - 1
	for i, v := range data {
		value += pow2(len-i) * v
	}
	return value
}
func pow2(n int) int {
	value := 1
	for i := 0; i < n; i++ {
		value *= 2
	}
	return value
}
