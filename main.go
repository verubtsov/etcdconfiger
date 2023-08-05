package etcdconfiger

import (
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdConfiger struct {
	endpoints    []string
	pathToFolder string

	client         *clientv3.Client
	sessionTimeout time.Duration

	namespaces      map[string]*Namespace
	callbacks       map[string]func([]string)
	keyNamespaceMap map[string][]*Namespace
	logger          LoggerTemplate
}

var (
	endpoints    = strings.Split(envString("ETCD_ADDRESS", "127.0.0.1:2379"), ",")
	pathToFolder = envString("ETCD_PATH", "/")
	deleteUnused = envBool("ETCD_DELETE_UNUSED", false)
)

func (e *EtcdConfiger) NewConfig(endpoints []string, pathToFolder string, log LoggerTemplate) {

}
