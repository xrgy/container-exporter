package config

import (
	_ "github.com/go-sql-driver/mysql"
	"os"
	"log"
	"time"
	"encoding/json"
	"database/sql"
	"github.com/coreos/etcd/client"
	"github.com/ghodss/yaml"
	"context"
	yaml2 "gopkg.in/yaml.v2"
	"strings"
)

var db *sql.DB

type Spec struct {
	Ports     map[string]string `yaml:"ports"`
	Selector  map[string]string `yaml:"selector"`
	ClusterIP string            `yaml:"clusterIP"`
	Stype     string            `yaml:"type"`
}
type ETCDParameter struct {
	Kind     string            `yaml:"kind"`
	Metadata map[string]string `yaml:"metadata"`
	Spec     Spec              `yaml:"spec"`
	Status   map[string]string `yaml:"status"`
}

type ConnectInfoData struct {
	IP          string
	Params_maps map[string]string
}
type Monitor_info []byte
type ConnectInfo struct {
	ip     string
	m_info Monitor_info
}

func readEtcdInfo(cfg client.Config, servicename string) string {
	c, err := client.New(cfg)
	if err != nil {
		log.Printf("%s", err.Error())
		return ""
	}
	//m := make(map[string]string)
	kapi := client.NewKeysAPI(c)
	resp1, err := kapi.Get(context.Background(), "/registry/services/specs/default/"+servicename, nil)
	if err != nil {
		return ""
	} else {
		log.Printf("etcd node value:" + resp1.Node.Value)
		param := &ETCDParameter{}
		v_rw := []byte(resp1.Node.Value)
		y_rw, err := yaml.JSONToYAML(v_rw)
		if err != nil {
			log.Printf("%s", err.Error())
			return ""
		}

		yaml2.Unmarshal(y_rw, &param)
		return param.Spec.ClusterIP

	}

}
func GetDBHandle() *sql.DB {
	var err error
	cfg := client.Config{
		Endpoints:               []string{"http://" + os.Getenv("ETCD_ENDPOINT")},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	DBUsername := os.Getenv("DB_USERNAME")
	DBPassword := os.Getenv("DB_PASSWORD")
	DBEndpoint := os.Getenv("DB_ENDPOINT")
	DBDatabase := os.Getenv("DB_DATABASE")
	servicename := strings.Split(DBEndpoint, ":")[0]
	serviceport := strings.Split(DBEndpoint, ":")[1]
	ip := readEtcdInfo(cfg, servicename)
	dsn := DBUsername + ":" + DBPassword + "@(" + ip + ":" + serviceport + ")/" + DBDatabase
	//dsn := DBUsername + ":" + DBPassword + "@(" + DBEndpoint + ")/" + DBDatabase
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("get DB handle error: %v", err)
	}
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(28000 * time.Second)
	err = db.Ping()
	if err != nil {
		log.Printf("connecting DB error: %v ", err)
	}
	return db
}
func GetMonitorInfo(id string) ConnectInfoData {
	info := queryConnectInfo(id)
	m := info.m_info
	m_info_map := make(map[string]string)
	if len(m) != 0 {
		err := json.Unmarshal(m, &m_info_map)
		if err != nil {
			log.Printf("Unmarshal error")
		}
	}
	con_info_data := ConnectInfoData{
		info.ip,
		m_info_map,
	}
	return con_info_data
}
func queryConnectInfo(id string) ConnectInfo {
	rows, err := db.Query("select ip,monitor_info from tbl_monitor_record where uuid=?", id)
	if err != nil {
		log.Printf("query error")
	}
	info := ConnectInfo{}
	for rows.Next() {
		err = rows.Scan(&info.ip, &info.m_info)
	}
	defer rows.Close()
	return info
}
func CloseDBHandle() {
	db.Close()
}
