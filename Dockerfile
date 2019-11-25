FROM ufoym/deepo:all-py36-cu100

ADD src/pod-gpu-metrics-exporter /usr/bin/pod-gpu-metrics-exporter

ENTRYPOINT ["pod-gpu-metrics-exporter", "-logtostderr", "-v", "8"]
