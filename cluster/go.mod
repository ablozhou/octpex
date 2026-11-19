module octpex/cluster

go 1.26.5

require (
	github.com/nats-io/nats.go v1.39.1
	github.com/redis/go-redis/v9 v9.7.0
	go.etcd.io/etcd/client/v3 v3.5.17
	octpex v0.0.0
)

replace octpex => ..
