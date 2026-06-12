# Регистрация времени отсутствия

## Необходимые пакеты

go get github.com/go-chi/chi/v5
go get github.com/joho/godotenv
go get github.com/sijms/go-ora/v2
go get github.com/jmoiron/sqlx
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
go get github.com/xuri/excelize/v2
go get github.com/natefinch/lumberjack
go mod tidy


## Стартуем prometeus

### Докер Grafana

docker run -d \
  -p 9090:9090 \
  -v /path/to/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus

### Проверяем работоспособность Grafana

http://localhost:9090/targets
