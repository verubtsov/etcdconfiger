package etcdconfiger

import (
	"context"
	"fmt"
	"reflect"
	"sort"
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
			e.logger.Printf("WARN: |EtcdConfiger| Cannot use private fields from config to asign it to params from etcd: %s\n", configElemName)
			continue
		}

		if !configElem.Value.IsValid() {
			e.logger.Printf("WARN: |EtcdConfiger| Invalid field: %s\n", configElemName)
			continue
		}

		configElem.Assignable = true

		ns.EtcdPaths = append(ns.EtcdPaths, e.pathToFolder)
		ns.Fields[configElemName] = configElem
	}
	return
}

func NewEtcdConfiger(log LoggerTemplate) *EtcdConfiger {
	e := &EtcdConfiger{
		endpoints:    endpoints,
		pathToFolder: pathToFolder,

		sessionTimeout: sessionTimeout,

		callbacks:       make(map[string]func([]string)),
		namespaces:      make(map[string]*Namespace),
		keyNamespaceMap: map[string][]*Namespace{},
	}

	e.logger.Printf("INFO: |EtcdConfiger| Connecting to ETCD endpoints\n")
	var err error
	e.client, err = clientv3.New(clientv3.Config{
		Endpoints:   e.endpoints,
		DialTimeout: e.sessionTimeout,
	})
	if err != nil {
		e.logger.Fatalf("ERROR: |EtcdConfiger| Failed connecting to endpoints\n")
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

	for _, param := range e.namespaces[namespace].Fields {
		values := make([]Update, 0, len(param.ETCDValues))

		for _, v := range param.ETCDValues {
			values = append(values, v)
		}

		if len(values) > 0 {
			sort.Slice(values, func(i, j int) bool {
				return values[i].Level > values[j].Level
			})
			//debug logger

			if !param.Assignable {
				return
			}

			param.setValueFromString(values[0].Value)
		} else {
			var defaultValue, key string
			if parsedDefaultValue, ok := param.StructField.Tag.Lookup("default"); ok {
				defaultValue = parsedDefaultValue
			} else {
				defaultValue = string(param.StructField.Tag)
			}

			key = fmt.Sprintf("%s/%s", e.pathToFolder, param.Name)

			//debug print
			ctx, _ := context.WithTimeout(context.Background(), e.sessionTimeout)
			_, err := e.client.Put(ctx, key, defaultValue)
			if err != nil {
				e.logger.Fatal(err)
			} else {
				param.setValueFromString(defaultValue)
			}
		}
	}

	if deleteUnused {
		e.deleteUnusedParam(namespace)
	}
}

func (e *EtcdConfiger) readFromEtcdPath(namespace string, path string) {
	ctx, _ := context.WithTimeout(context.Background(), e.sessionTimeout)

	response, err := e.client.Get(ctx, path, clientv3.WithPrefix())
	if err != nil {
		e.logger.Fatalf("FATAL: |EtcdConfigurer| Error while reading values %+v\n", err)
	}

	namespaceData := e.namespaces[namespace]

	for _, respValues := range response.Kvs {
		update := newUpdate(string(respValues.Key), string(respValues.Value))

		e.logger.Printf("INFO: |EtcdConfigurer| Recieved parameter: \"%s\", value: \"%s\", path: \"%s\"\n", update.ParamName, update.Value, update.Key)

		if _, ok := namespaceData.Fields[update.ParamName]; !ok {
			e.logger.Printf("UPDATE: |EtcdConfigurer| Unknown parameter \"%s\"\n", update.ParamName)
		} else {
			namespaceData.Fields[update.ParamName].ETCDValues[update.Level] = update
		}
	}
	e.namespaces[namespace] = namespaceData
}

func (e *EtcdConfiger) deleteUnusedParam(namespace string) {
	configParams := e.namespaces[namespace].Fields
	ctx, _ := context.WithTimeout(context.Background(), e.sessionTimeout)
	resp, err := e.client.Get(ctx, e.pathToFolder, clientv3.WithPrefix())
	if err != nil {
		e.logger.Fatalf("ERROR: |EtcdConfiger| Error while reading values: %s\n", err)
	}
	var paramsToSave []string
	for _, ev := range resp.Kvs {
		field := newUpdate(string(ev.Key), string(ev.Value))
		for _, param := range configParams {
			if param.Name == field.ParamName {
				paramsToSave = append(paramsToSave, param.Name)
			}
		}
		flagForDelete := true
		for _, str := range paramsToSave {
			if str == field.ParamName {
				flagForDelete = false
			}
		}
		if flagForDelete {
			_, err := e.client.Delete(ctx, field.Key)
			if err != nil {
				e.logger.Fatalf("ERROR: |EtcdConfiger| Error while delete values: %s\n", err)
			}
			e.logger.Printf("INFO: [EtcdConfiger] delete param: %s\n", field.Key)
		}
	}
}

func (field *ConfigField) setValueFromString(value string) (err error) {
	cv := NewEtcdValue(value)
	switch field.StructField.Type.Kind() {
	case reflect.Bool:
		b, err := cv.Bool()
		if err == nil {
			field.Value.SetBool(b)
		}
	case reflect.String:
		field.Value.SetString(value)
	case reflect.Int, reflect.Int32, reflect.Int64:
		duration, err := cv.TimeDuration()
		if err == nil {
			field.Value.SetInt(int64(duration))
			return nil
		}
		i, err := cv.Int()
		if err == nil {
			field.Value.SetInt(int64(i))
		}
	case reflect.Float32, reflect.Float64:
		f, err := cv.Float()
		if err == nil {
			field.Value.SetFloat(f)
		}
	case reflect.Slice:
		switch field.StructField.Type.Elem().Kind() {
		case reflect.String:
			field.Value.Set(reflect.ValueOf(cv.Strings()))
		}
	default:
		err = fmt.Errorf("UNKNOWN CONFIG TYPE: %v", field.StructField.Type.Kind())
	}
	return
}
