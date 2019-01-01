package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"container-exporter/config"
	"log"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/google/cadvisor/client/v2"
)

type K8sNodeCollector struct {
	Target string
}
func (c K8sNodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}
var (
	node_label             = []string{"ip", "nodelabel"}
	k8s_node_cpu_usage     = prometheus.NewDesc("k8s_node_cpu_usage", "node cpu usage in percent", node_label, nil)
	k8s_node_memory_used   = prometheus.NewDesc("k8s_node_memory_used", "node memory used in bytes", node_label, nil)
	k8s_node_memory_total  = prometheus.NewDesc("k8s_node_memory_total", "node memory total in bytes", node_label, nil)
	k8s_node_memory_avlil  = prometheus.NewDesc("k8s_node_memory_avlil", "node memory available in bytes", node_label, nil)
	k8s_node_monitorstatus = prometheus.NewDesc("k8s_node_monitorstatus", "k8s node monitor status", nil, nil)
	k8s_node_status = prometheus.NewDesc("k8s_node_status", "k8s node status", node_label, nil)
	k8s_node_container_total = prometheus.NewDesc("k8s_node_container_total", "k8s node containers in total", node_label, nil)
	k8s_node_uptime = prometheus.NewDesc("k8s_node_uptime", "k8s node up time", node_label, nil)

)

func (c K8sNodeCollector) Collect(ch chan<- prometheus.Metric) {
	monitor_info := config.GetMonitorInfo(c.Target)
	master_IP := monitor_info.Params_maps["master_ip"]
	nodeIp := monitor_info.Params_maps["node_ip"]
	aport := monitor_info.Params_maps["api_port"]
	nodename := monitor_info.Params_maps["node_name"]
	cport := monitor_info.Params_maps["cadvisor_port"]
	endpoint := master_IP + ":" + aport
	config := &rest.Config{
		Host: "http://" + endpoint,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("get clientset error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_node_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	node, err := clientset.CoreV1().Nodes().Get(nodename, metav1.GetOptions{})
	if err != nil {
		log.Printf("get node error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_node_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	label := node.Labels["node"]
	var status =""
	for _,v := range node.Status.Conditions{
		if v.Type == "Ready" {
			status = string(v.Status)
			break
		}
	}
	var statuscode =0
	if status == "True" {
		statuscode = 1
	}else {
		statuscode = 0
	}
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		log.Printf("get pods list error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_node_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	var containercount = 0
	for _,v := range pods.Items{
		if v.Spec.NodeName == nodename {
			cons := v.Spec.Containers
			containercount = containercount+len(cons)
		}
	}
	createtime := node.CreationTimestamp.Unix()
	labelvalues := []string{nodeIp, label}
	ch <- prometheus.MustNewConstMetric(k8s_node_status, prometheus.GaugeValue, float64(statuscode),labelvalues...)
	ch <- prometheus.MustNewConstMetric(k8s_node_container_total, prometheus.GaugeValue, float64(containercount),labelvalues...)
	ch <- prometheus.MustNewConstMetric(k8s_node_uptime, prometheus.GaugeValue, float64(createtime),labelvalues...)
	cadvisorendpoint := nodeIp + ":" + cport
	client, err := v2.NewClient("http://" + cadvisorendpoint)
	if err != nil {
		log.Printf("get client error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_node_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	ms, err := client.MachineStats()
	if err != nil {
		log.Printf("get machine stats error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_node_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	length := len(ms)
	latest := ms[length-1]//倒数第一个
	secondlatest := ms[length-2]//倒数第二个
	deltatime := latest.Timestamp.UnixNano() - secondlatest.Timestamp.UnixNano()
	deltacputime := int64(latest.Cpu.Usage.Total - secondlatest.Cpu.Usage.Total)
	core := node.Status.Capacity.Cpu().Value()
	cpuusage := float64(100 * deltacputime / (core * deltatime))
	ch <- prometheus.MustNewConstMetric(k8s_node_cpu_usage, prometheus.GaugeValue, float64(cpuusage),labelvalues...)
	memoryused := float64(latest.Memory.Usage)
	ch <- prometheus.MustNewConstMetric(k8s_node_memory_used, prometheus.GaugeValue, float64(memoryused),labelvalues...)
	totalmemory := node.Status.Capacity.Memory().Value()
	ch <- prometheus.MustNewConstMetric(k8s_node_memory_total, prometheus.GaugeValue, float64(totalmemory),labelvalues...)
	ch <- prometheus.MustNewConstMetric(k8s_node_memory_avlil, prometheus.GaugeValue, float64(totalmemory)-memoryused,labelvalues...)
	fsstate := latest.Filesystem
	node_label=append(node_label,"device")
	k8s_node_filesystem_total:= prometheus.NewDesc("k8s_node_filesystem_total", "k8s node filesystem in total", node_label, nil)
	k8s_node_filesystem_used := prometheus.NewDesc("k8s_node_filesystem_used", "k8s node filesystem in used", node_label, nil)
	k8s_node_filesystem_avail := prometheus.NewDesc("k8s_node_filesystem_avail", "k8s node filesystem in avail", node_label, nil)

	for _,state := range fsstate {
		capacity := state.Capacity
		used := state.Usage
		avail := state.Available
		ch <- prometheus.MustNewConstMetric(k8s_node_filesystem_total, prometheus.GaugeValue, float64(*capacity),append(labelvalues,state.Device)...)
		ch <- prometheus.MustNewConstMetric(k8s_node_filesystem_used, prometheus.GaugeValue, float64(*used),append(labelvalues,state.Device)...)
		ch <- prometheus.MustNewConstMetric(k8s_node_filesystem_avail, prometheus.GaugeValue, float64(*avail),append(labelvalues,state.Device)...)
	}
	ch <- prometheus.MustNewConstMetric(k8s_container_monitorstatus, prometheus.GaugeValue, float64(1))
}
