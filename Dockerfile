FROM        quay.io/prometheus/busybox:latest
USER root


COPY container-exporter                          /bin/container-exporter
#ADD .                         $GOPATH/src/db-exporter
#WORKDIR $GOPATH/src/db-exporter


#RUN go build .
EXPOSE     9109
ENTRYPOINT ["/bin/container-exporter"]
