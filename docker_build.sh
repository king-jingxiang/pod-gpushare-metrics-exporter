#!/usr/bin/env bash

cd ${GOPATH}/src/github.com/ruanxingbaozi/pod-gpu-metrics-exporter/src
go build -o pod-gpu-metrics-exporter -v github.com/ruanxingbaozi/pod-gpu-metrics-exporter/src

cd ${GOPATH}/src/github.com/ruanxingbaozi/pod-gpu-metrics-exporter
sudo docker build -t pcl/pod-gpu-metrics-exporter:v1.0.0-alpha .

kubectl apply -f pod-gpu-metrics-exporter-daemonset.yaml

kubectl get po -nkube-system | grep pod-gpu-metrics-exporter | awk '{print $1}' | xargs kubectl delete po -nkube-system

sleep 3
kubectl get po -nkube-system | grep pod-gpu-metrics-exporter | awk '{print $1}' | xargs kubectl logs -f -nkube-system -c pod-nvidia-gpu-metrics-exporter


# kubectl exec -it $(kubectl get po -nkube-system | grep pod-gpu-metrics-exporter | awk '{print $1}') -nkube-system -c pod-nvidia-gpu-metrics-exporter bash