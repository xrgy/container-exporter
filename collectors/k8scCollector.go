package collectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/google/cadvisor/info/v1"
	"container-exporter/config"
	"github.com/google/cadvisor/client"
	"log"
	"regexp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
	"fmt"
)

type K8sContainerCollector struct {
	Target string
}
func (c K8sContainerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}
var containerMetrics = []containerMetric{
	{
		name:"k8s_container_last_seen",
		help:"Last time a container was seen by the exporter",
		valueType:prometheus.GaugeValue,
		getValues: func(s *v1.ContainerStats) metricValues {
			return metricValues{{value:float64(time.Now().Unix())}}
		},
	},{
		name:"k8s_container_cpu_usage_seconds_total",
		help:"Cumulative cpu time consumed per cpu in seconds",//累积
		valueType:prometheus.CounterValue,
		extraLabels:[]string{"cpu"},
		getValues: func(s *v1.ContainerStats) metricValues {
			values := make(metricValues,0,len(s.Cpu.Usage.PerCpu))
			for i,value := range s.Cpu.Usage.PerCpu{
				values = append(values,metricValue{
					value:float64(value)/float64(time.Second),
					labels: []string{fmt.Sprintf("cpu%02d",i)},
				})
			}
			return values
		},
	},{
		name:"k8s_container_memory_usage_bytes",
		help:"Current memory usage in bytes",
		valueType:prometheus.GaugeValue,
		getValues: func(s *v1.ContainerStats) metricValues {
			return metricValues{{value:float64(s.Memory.Usage)}}
		},
	},
	{
		name:"k8s_container_fs_limit_bytes",
		help:"Number of bytes that can be consumed by the container on this filesystem",
		valueType:prometheus.GaugeValue,
		extraLabels:[]string{"device"},
		getValues: func(s *v1.ContainerStats) metricValues {
			return fsValues(s.Filesystem, func(fs *v1.FsStats) float64 {
				return float64(fs.Limit)
			})
		},
	},
	{
		name:"k8s_container_fs_usage_bytes",
		help:"Number of bytes that are consumed by the container on this filesystem",
		valueType:prometheus.GaugeValue,
		extraLabels:[]string{"device"},
		getValues: func(s *v1.ContainerStats) metricValues {
			return fsValues(s.Filesystem, func(fs *v1.FsStats) float64 {
				return float64(fs.Usage)
			})
		},
	},
}

func fsValues(fsStats []v1.FsStats, valueFn func(fs *v1.FsStats) float64) metricValues {
	values := make(metricValues,0,len(fsStats))
	for _,stat := range fsStats{
		values = append(values,metricValue{
			value: valueFn(&stat),
			labels:[]string{stat.Device},
		})
	}
	return values
}
const maxMemorySize  = uint64(1 << 62)
func specMemoryValue(v uint64) float64 {
	if v > maxMemorySize {
		return 0
	}
	return float64(v)
}
type containerMetric struct{
	name string
	help string
	valueType prometheus.ValueType
	extraLabels []string
	getValues func(s *v1.ContainerStats) metricValues
}
type metricValues []metricValue
type metricValue struct {
	value float64
	labels []string
}
var k8s_container_monitorstatus =prometheus.NewDesc("k8s_container_monitorstatus",
	"k8s container monitor status ",nil,nil)

func (cm *containerMetric) desc(baseLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(cm.name,cm.help,append(baseLabels,cm.extraLabels...),nil)
}
func (c K8sContainerCollector)Collect(ch chan<-prometheus.Metric ) {
	monitor_info := config.GetMonitorInfo(c.Target)
	nodeIp:= monitor_info.Params_maps["node_ip"]
	masterIp:= monitor_info.Params_maps["master_ip"]
	contaninerID:=monitor_info.Params_maps["container_id"]
	podname := monitor_info.Params_maps["pod_name"]
	podnamespace := monitor_info.Params_maps["pod_namespace"]
	aport := monitor_info.Params_maps["api_port"]
	cport := monitor_info.Params_maps["cadvisor_port"]
	cadvisorendpoint := nodeIp+":"+cport
	client,err := client.NewClient("http://"+cadvisorendpoint)
	if err!=nil {
		log.Printf("get client error: %s",err.Error())
		ch<-prometheus.MustNewConstMetric(k8s_container_monitorstatus,prometheus.GaugeValue,float64(0))
		return
	}
	request := v1.ContainerInfoRequest{NumStats:1}
	cinfo,err2 := client.DockerContainer(contaninerID,&request)
	if err2!=nil {
		log.Printf("get containerinfo error: %s",err2.Error())
		ch<-prometheus.MustNewConstMetric(k8s_container_monitorstatus,prometheus.GaugeValue,float64(0))
		return
	}
	baseLabels := []string{"id"}
	id:=cinfo.Name
	name:=id
	if len(cinfo.Aliases) >0{
		name = cinfo.Aliases[0]
		baseLabels = append(baseLabels,"name")
	}
	baseLabels = append(baseLabels,"nodeIP")
	baseLabelValues := []string{id,name,nodeIp}[:len(baseLabels)]
	newLabels := containerNameToLabels(name)
	for k,v := range newLabels{
		baseLabels =append(baseLabels,k)
		baseLabelValues = append(baseLabelValues,v)
	}
	minfo,err := client.MachineInfo()
	if err!=nil {
		log.Printf("get machineinfo error: %s",err.Error())
		ch<-prometheus.MustNewConstMetric(k8s_container_monitorstatus,prometheus.GaugeValue,float64(0))
	}
	core := minfo.NumCores
	memory := minfo.MemoryCapacity
	machine_core := prometheus.NewDesc("k8s_container_machine_cores","Number of CPU cores on this node",
		[]string{"nodeIP"},nil)
	ch<-prometheus.MustNewConstMetric(machine_core,prometheus.GaugeValue,float64(core),[]string{nodeIp}...)
	machine_memory := prometheus.NewDesc("k8s_container_machine_memory","Amount of memory installed" +
		"on the node",[]string{"nodeIP"},nil)
	ch<-prometheus.MustNewConstMetric(machine_memory,prometheus.GaugeValue,float64(memory),[]string{nodeIp}...)

	containername := newLabels["kubernetes_container_name"]
	desc := prometheus.NewDesc("k8s_container_start_time_seconds", "start time of the container since unix epoch in seconds", baseLabels, nil)
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(cinfo.Spec.CreationTime.Unix()),baseLabelValues...)
	if cinfo.Spec.HasCpu {
		desc = prometheus.NewDesc("k8s_container_spec_cpu_period", "cpu period of the container", baseLabels, nil)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(cinfo.Spec.Cpu.Period),baseLabelValues...)
		if cinfo.Spec.Cpu.Quota !=0{
			desc = prometheus.NewDesc("k8s_container_spec_cpu_quota", "cpu quota of the container", baseLabels, nil)
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(cinfo.Spec.Cpu.Quota),baseLabelValues...)
		}
		desc = prometheus.NewDesc("k8s_container_spec_cpu_shares", "cpu share of the container", baseLabels, nil)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(cinfo.Spec.Cpu.Limit),baseLabelValues...)
	}
	if cinfo.Spec.HasMemory {
		desc = prometheus.NewDesc("k8s_container_spec_memory_limit_bytes", "memory limit for the container", baseLabels, nil)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, specMemoryValue(cinfo.Spec.Memory.Limit),baseLabelValues...)
		desc = prometheus.NewDesc("k8s_container_spec_memory_swap_limit_bytes", "memory swap limit for the container", baseLabels, nil)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, specMemoryValue(cinfo.Spec.Memory.SwapLimit),baseLabelValues...)
	}
	stats := cinfo.Stats[0]
	k8sendpoint := masterIp +":"+aport
	config := &rest.Config{
		Host:"http://" + k8sendpoint,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("get clientset error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_container_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	pod, err := clientset.CoreV1().Pods(podnamespace).Get(podname,metav1.GetOptions{})
	if err != nil {
		log.Printf("get pods list error: %s", err.Error())
		ch <- prometheus.MustNewConstMetric(k8s_container_monitorstatus, prometheus.GaugeValue, float64(0))
		return
	}
	containerstatus := pod.Status.ContainerStatuses
	for _,v := range containerstatus{
		if v.Name == containername {
			var containerstate = 3
			if v.State.Waiting !=nil {
				containerstate = 0
			}else {
				if v.State.Running !=nil {
					containerstate = 1
				}else {
					containerstate = 2
				}
			}
			restartcount := v.RestartCount
			container_state := prometheus.NewDesc("k8s_container_state", "container state,0:wating,1:runing,2:terminated", baseLabels, nil)
			ch <- prometheus.MustNewConstMetric(container_state, prometheus.GaugeValue, float64(containerstate),baseLabelValues...)
			container_restart := prometheus.NewDesc("k8s_container_restart", "container restart times", baseLabels, nil)
			ch <- prometheus.MustNewConstMetric(container_restart, prometheus.GaugeValue, float64(restartcount),baseLabelValues...)
			break
		}
	}
	for _,cm := range containerMetrics{
		desc := cm.desc(baseLabels)
		for _,metricValue := range cm.getValues(stats){
			ch<-prometheus.MustNewConstMetric(desc,prometheus.GaugeValue,float64(metricValue.value),append(baseLabels,metricValue.labels...)...)
		}
	}
	ch<-prometheus.MustNewConstMetric(k8s_container_monitorstatus,prometheus.GaugeValue,float64(1))
}

func containerNameToLabels(name string) map[string]string {
	re := regexp.MustCompile(`^k8s_(?P<kubernetes_container_name>[^_\.]+)[^_]*_(?P<kubernetes_pod_name>[^_]+)_(?P<kubernetes_namespace>[^_]+)`)
	reCaptureNames := re.SubexpNames()
	extraLabels := map[string]string{}
	matches := re.FindStringSubmatch(name)
	for i,match := range matches{
		if len(reCaptureNames[i])>0 {
			extraLabels[re.SubexpNames()[i]] = match
		}
	}
	return extraLabels
}