#!/usr/bin/env bash


sudo docker build -t pcl/podpod-gpu-metrics-exporter:v1.0.0-alpha .


go build -o src/main.go