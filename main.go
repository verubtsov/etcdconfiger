package etcdconfiger

import (
	"context"
	"reflect"
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
	endpoints      = strings.Split(envString("ETCD_ADDRESS", "127.0.0.1:2379"), ",")
	pathToFolder   = envString("ETCD_PATH", "/")
	deleteUnused   = envBool("ETCD_DELETE_UNUSED", false)
	sessionTimeout = envDuration("ETCD_SESSION_TIMEOUT", 5*time.Second)
)

func (e *EtcdConfiger) namespaceInitialization(name string, config interface{}, callback func(param string, prev EtcdValue)) (ns *Namespace) {
	ns = &Namespace{
		Name:      name,
		Fields:    make(map[string]ConfigField),
		EtcdPaths: []string{},
		Callback:  callback,
	}

	configReflect := reflect.ValueOf(config)
	if configReflect.Kind() == reflect.Ptr {
		configReflect = reflect.Indirect(configReflect)
	}

	for i := 0; i < configReflect.NumField(); i++ {
		configElem := ConfigField{
			StructField: configReflect.Type().Field(i),
			Value:       configReflect.Field(i),
			ETCDValues:  make(map[int]Update),
		}

		configElemName := configElem.StructField.Name

		if !configElem.Value.CanSet() {
			e.logger.Printf("WARN: |EtcdConfiger| Cannot use private fields from config to asign it to params from etcd: %s", configElemName)
			continue
		}

		if !configElem.Value.IsValid() {
			e.logger.Printf("WARN: |EtcdConfiger| Invalid field: %s", configElemName)
			continue
		}

		configElem.Assignable = true

		ns.EtcdPaths = append(ns.EtcdPaths, e.pathToFolder)
		ns.Fields[configElemName] = configElem
	}
	return
}

func NewEtcdConfiger(pathToFolder string, log LoggerTemplate) *EtcdConfiger {
	e := &EtcdConfiger{
		endpoints:    endpoints,
		pathToFolder: pathToFolder,

		sessionTimeout: sessionTimeout,

		callbacks:       make(map[string]func([]string)),
		namespaces:      make(map[string]*Namespace),
		keyNamespaceMap: map[string][]*Namespace{},
	}

	e.logger.Printf("INFO: |EtcdConfiger| Connecting to ETCD endpoints")
	var err error
	e.client, err = clientv3.New(clientv3.Config{
		Endpoints:   e.endpoints,
		DialTimeout: e.sessionTimeout,
	})
	if err != nil {
		e.logger.Fatalf("ERROR: |EtcdConfiger| Failed connecting to endpoints")
	}

	return e
}

func (e *EtcdConfiger) Configure(namespace string, conf interface{}, callback func(param string, prev EtcdValue)) {
	e.namespaces[namespace] = e.namespaceInitialization(namespace, conf, callback)

	for _, etcdPath := range e.namespaces[namespace].EtcdPaths {
		e.readFromEtcdPath(namespace, etcdPath)

		if _, ok := e.keyNamespaceMap[etcdPath]; !ok {
			e.keyNamespaceMap[etcdPath] = make([]*Namespace, 0)
		}
		e.keyNamespaceMap[etcdPath] = append(e.keyNamespaceMap[etcdPath], e.namespaces[namespace])
	}
	
}

func (e *EtcdConfiger) readFromEtcdPath(namespace string, path string) {
	ctx, _ := context.WithTimeout(context.Background(), e.sessionTimeout)

	response, err := e.client.Get(ctx, path, clientv3.WithPrefix())
	if err != nil {
		e.logger.Fatalf("FATAL: |EtcdConfigurer| Error while reading values %+v", err)
	}

	namespaceData := e.namespaces[namespace]

	for _, respValues := range response.Kvs {
		update := Update{
			Key:   string(respValues.Key),
			Value: string(respValues.Value),
		}
		paramName := strings.Split(update.Key, "/")
		update.ParamName = paramName[len(paramName)-1]
		update.Level = len(paramName) - 1

		e.logger.Printf("INFO: |EtcdConfigurer| Recieved parameter: \"%s\", value: \"%s\", path: \"%s\"", update.ParamName, update.Value, update.Key)

		if _, ok := namespaceData.Fields[update.ParamName]; !ok {
			e.logger.Printf("UPDATE: |EtcdConfigurer| Unknown parameter \"%s\"", update.ParamName)
		} else {
			namespaceData.Fields[update.ParamName].ETCDValues[update.Level] = update
		}
	}
	e.namespaces[namespace] = namespaceData
}
