FROM ubuntu:16.04

ADD pod-gpu-metrics-exporter /usr/bin/pod-gpu-metrics-exporter

ENTRYPOINT ["pod-gpu-metrics-exporter", "-logtostderr", "-v", "8"]
