package etcdconfiger

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

type LoggerTemplate interface {
	Printf(string, ...any)
	Fatalf(string, ...any)
	Panicf(string, ...any)
	Fatal(...any)
}

type EtcdValue struct {
	value string
}

func (e *EtcdValue) NewEtcdValue(value string) EtcdValue {
	return EtcdValue{
		value: value,
	}
}

func (e *EtcdValue) String() string {
	return e.value
}

func (e *EtcdValue) TimeDuration() (time.Duration, error) {
	return time.ParseDuration(e.value)
}

func (e *EtcdValue) Bool() (bool, error) {
	return strconv.ParseBool(e.value)
}

func (e *EtcdValue) Int() (int, error) {
	return strconv.Atoi(e.value)
}

func (e *EtcdValue) Float() (float64, error) {
	return strconv.ParseFloat(e.value, 64)
}

func (e *EtcdValue) Strings() []string {
	return strings.Split(e.value, "\n")
}

type Namespace struct {
	Name     string
	Fields   map[string]EtcdValue
	EtcdKeys []string
	Callback func(paramName string, prevValue EtcdValue)
}

type Update struct {
	IsUpdate              bool
	Key, Value, ParamName string
	Level                 int
}

type ConfigField struct {
	Name        string
	Value       reflect.Value
	StructField reflect.StructField
	ETCDValues  map[int]Update
	Assignable  bool
}
