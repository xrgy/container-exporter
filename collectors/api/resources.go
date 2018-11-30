package api

import (
	"net/http"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	"log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	"strings"
	"encoding/json"
)
type Container struct {
	Id string 	`json:"id"`
	Name string `json:"name"`
	Status string `json:"status"`
}
type Pod struct {
	Name string	`json:"name"`
	Namespace string `json:"namespace"`
	Containers []Container `json:"containers"`
}
type Node struct {
	Ip string	`json:"ip"`
	Name string `json:"name"`
	Pods []Pod 	`json:"pods"`
}
type Resource struct {
	Nodes []Node `json:"nodes"`
}

func GetContainerList(w http.ResponseWriter, r *http.Request) {
	master_ip:=r.URL.Query().Get("master_ip")
	aport:=r.URL.Query().Get("api_port")
	if master_ip=="" ||aport==""  {
		http.Error(w,"invalid request parameter",400)
		return
	}
	endpoint := master_ip + ":" + aport
	config := &rest.Config{
		Host: "http://" + endpoint,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("get clientset error: %s", err.Error())
		http.Error(w,"create clientset error ",500)
		return
	}
	nodelist, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		log.Printf("get node list error: %s", err.Error())
		http.Error(w,"cannot get nodes",404)
		return
	}
	var nodes=[]Node{}
	for _,v :=range nodelist.Items{
		nodeip := getNodeIP(v)
		if nodeip=="" {
			log.Printf("cannot get IP of the node:%s",v.Name)
			continue
		}
		pods:=getPodInfo(clientset,nodeip)
		node:=Node{nodeip,v.Name,pods,}
		nodes=append(nodes,node)
	}
	resource:=Resource{nodes,}
	w.Header().Set("Content-Type","application/json")
	json.NewEncoder(w).Encode(resource)
}
func getNodeIP(node v1.Node) string {
	nodeaddresss:=node.Status.Addresses
	for _,v :=range nodeaddresss{
		if v.Type =="InternalIP" {
			return v.Address
		}
	}
	return ""
}
func getPodInfo(clientset *kubernetes.Clientset,nodeip string) []Pod{
	var newPods []Pod
	pods,err:=clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err!=nil {
		log.Printf("get pods error:%s",err.Error())
		return nil
	}
	for _,p:=range pods.Items{
		if p.Status.HostIP == nodeip {
			podname :=p.Name
			containerinfo := p.Status.ContainerStatuses
			var cons =[]Container{}
			for _,c:=range containerinfo{
				if c.ContainerID =="" {
					log.Printf("no container id in pod:%s",p.Name)
					continue
				}
				containerid:= convertContainerID(c.ContainerID)
				containername:=c.Name
				containerstatus:=getContainerStatus(c.State)
				con:=Container{containerid,containername,containerstatus,}
				cons=append(cons,con)
			}
			pod:=Pod{podname,p.Namespace,cons,}
			newPods=append(newPods,pod)
		}
	}
	return newPods
}
func convertContainerID(id string) string {
	s:=strings.Split(id,"://")
	shortid:=s[1]
	return shortid
}
func getContainerStatus(state v1.ContainerState) string {
	var conatinerstate="3"
	if state.Waiting !=nil {
		conatinerstate="0"
	}else {
		if state.Running!=nil {
			conatinerstate="1"
		}else {
			conatinerstate="2"
		}
	}
	return conatinerstate
}
