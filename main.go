package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strings"
	"fmt"
	"container-exporter/collectors"
	"container-exporter/config"
	"github.com/gorilla/mux"
	"container-exporter/collectors/api"
	"log"
)
var listenAddress = kingpin.Flag("web.listen-address","Address to listen on for web " +
	"interface and telemetry.").Default(":9109").String()



func runCollector(collector prometheus.Collector,w http.ResponseWriter,r *http.Request)  {
	registry:= prometheus.NewRegistry()
	registry.MustRegister(collector)
	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		registry,
	}
	h:=promhttp.HandlerFor(gatherers,promhttp.HandlerOpts{})
	h.ServeHTTP(w,r)
}
func init() {
	config.GetDBHandle()
}
func main() {
	kingpin.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/k8s",handler)
	r.HandleFunc("/k8sc",handler)
	r.HandleFunc("/k8sn",handler)
	r.HandleFunc("/api/v1/resources",api.GetContainerList)
	http.ListenAndServe(*listenAddress,r)

}
func handler(w http.ResponseWriter,r *http.Request)  {
	var collectorType prometheus.Collector
	target:= r.URL.Query().Get("target")
	if target=="" {
		http.Error(w,"'target' parameter must be specified",400)
		return
	}
	atr:=strings.Split(fmt.Sprintf("%s",r.URL),"?")[0]
	log.Printf(atr)
	switch strings.Split(fmt.Sprintf("%s",r.URL),"?")[0] {
	case "/k8s":
		collectorType = collectors.K8sCollector{target}
		break
	case "/k8sc":
		collectorType = collectors.K8sContainerCollector{target}
		break
	case "/k8sn":
		collectorType = collectors.K8sNodeCollector{target}
		break
	default:
		break
	}

	runCollector(collectorType,w,r)
}