FROM debian:stretch-slim

ADD src/pod-gpu-metrics-exporter /usr/bin/pod-gpu-metrics-exporter

ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

ENTRYPOINT ["pod-gpu-metrics-exporter", "-logtostderr", "-v", "8"]
#ENTRYPOINT ["pod-gpu-metrics-exporter"]
