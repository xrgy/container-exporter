package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"container-exporter/config"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	"log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sCollector struct {
	Target string
}
var (
	k8s_cluster_nodes_total     = prometheus.NewDesc("k8s_cluster_nodes_total", "k8s cluster nodes in total", nil, nil)
	k8s_cluster_cpucores_total = prometheus.NewDesc("k8s_cluster_cpucores_total", "k8s cluster cpucores in total", nil, nil)
	k8s_cluster_monitorstatus  = prometheus.NewDesc("k8s_cluster_monitorstatus", "k8s cluster node monitor status", nil, nil)
	k8s_cluster_containers_total = prometheus.NewDesc("k8s_cluster_containers_total", "k8s cluster containers in total", nil, nil)
	k8s_cluster_memory_total  = prometheus.NewDesc("k8s_cluster_memory_total", "k8s cluster memory in total", nil, nil)
)
func (c K8sCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

func (c K8sCollector) Collect(ch chan<- prometheus.Metric) {
	monitor_info := config.GetMonitorInfo(c.Target)
	master_IP := monitor_info.Params_maps["master_ip"]
	aport := monitor_info.Params_maps["api_port"]
	endpoint := master_IP + ":" + aport
	config := &rest.Config{
		Host: "http://" + endpoint,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("get clientset error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_cluster_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	nodelist, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Printf("get node list error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_cluster_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		log.Printf("get pods list error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_cluster_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	var containercount float64 =0
	for _,v := range pods.Items{
		cons := v.Spec.Containers
		containercount = containercount + float64(len(cons))
	}
	var totalcore float64 = 0
	var totalmemory float64 = 0
	for _,v := range nodelist.Items{
		core := v.Status.Capacity.Cpu().Value()
		memory := v.Status.Capacity.Memory().Value()
		totalcore = totalcore + float64(core)
		totalmemory = totalmemory + float64(memory)
		}
	ch <- prometheus.MustNewConstMetric(k8s_cluster_nodes_total,prometheus.GaugeValue,float64(len(nodelist.Items)))
	ch <- prometheus.MustNewConstMetric(k8s_cluster_containers_total,prometheus.GaugeValue,containercount)
	ch <- prometheus.MustNewConstMetric(k8s_cluster_cpucores_total,prometheus.GaugeValue,totalcore)
	ch <- prometheus.MustNewConstMetric(k8s_cluster_memory_total,prometheus.GaugeValue,totalmemory)
	ch <- prometheus.MustNewConstMetric(k8s_cluster_monitorstatus,prometheus.GaugeValue,float64(1))
}