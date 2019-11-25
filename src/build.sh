#!/usr/bin/env bash




go build -o pod-gpu-metrics-exporter -v /go/src/github.com/ruanxingbaozi/pod-gpu-metrics-exporter/src

sudo ./pod-gpu-metrics-exporter -logtostderr -v 8