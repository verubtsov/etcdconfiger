# EtcdConfiger
Это библиотека для удобного конфигурирования сервисов с помощью ETCD.
## Параметры окружения
Имя параметра | Назначение | Значение по умолчанию | Пример значения
--- | --- | --- | ---
ETCD_ADDRESS | Список конечных точек к ETCD перечисленных через запятую | 127.0.0.1:2379 | ip1:port1,ip2:port2
ETCD_PATH | Общая часть всех ключей (путь до директории, где хранятся параметры) | / | /local/test
ETCD_DELETE_UNUSED | Удаление неиспользуемых параметров | true | false
ETCD_SESSION_TIMEOUT | Таймаут на подключение / получение значения параметра | 5*time.Second | time.Minute
## Пример использования
```go
package test_module

import (
	"log"

	"github.com/verubtsov/etcdconfiger"
)

type Config struct {
	TestString string  `default:"testtesttest"`
	TestInt    int     `default:"1234"`
	TestFloat  float32 `default:"1.34"`
}

func NewConfig(logf *log.Logger) *Config {
	config := &Config{}
	etcdconfiger.NewEtcdConfiger(logf).Configure("main", config, func(param string, prevValue etcdconfiger.EtcdValue) {
		logf.Fatal("|EtcdConfiger| Updated ETCD params, restart")
	})

	return config
}
```
При существовании параметра с ключом `ETCD_PATH/TestString` его значение будет присвоено полю структуры `Config.TestString`. Если этого параметра не существует, то он будет создан с значением по умолчанию, указанным в теге `Config.TestString`, в данном примере, со значением `testtesttest`. 

## Changelog
### v0.2.0
Модуль доделан до рабочего состояния