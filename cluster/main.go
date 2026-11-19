package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"octpex/internal/cli"
	"octpex/pkg/bus"
	"octpex/pkg/config"
	"octpex/pkg/driver"
	"octpex/pkg/ring"
	"octpex/pkg/substrate"
	"strings"
	"time"
)

func main() { cli.Main(connect) }

func connect(ctx context.Context, c *config.Config) (*driver.NetworkSubsystems, error) {
	startup, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	r := redis.NewClient(&redis.Options{Addr: c.Substrate.Endpoint})
	if err := r.Ping(startup).Err(); err != nil {
		r.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	nc, err := nats.Connect(c.Bus.URL, nats.Timeout(4*time.Second))
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("nats: %w", err)
	}
	js, err := nc.JetStream()
	vertical := strings.TrimSuffix(c.Bus.VerticalSubject, ".*")
	lateral := strings.TrimSuffix(c.Bus.LateralSubject, ".*")
	if err == nil {
		err = ensureStream(js, []string{vertical + ".*", lateral + ".*"})
	}
	if err != nil {
		nc.Close()
		r.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	ec, err := clientv3.New(clientv3.Config{Endpoints: c.Ring.Endpoints, DialTimeout: 4 * time.Second})
	if err != nil {
		nc.Close()
		r.Close()
		return nil, err
	}
	reg, err := ring.NewETCDCapabilityRegistry(&etcdBackend{Client: ec}, int64(c.Ring.LeaseTTLSeconds))
	if err != nil {
		ec.Close()
		nc.Close()
		r.Close()
		return nil, err
	}
	consumerCtx, stop := context.WithCancel(ctx)
	transport := bus.NewNATSNervousBus(&jetBus{nc: nc, js: js, ctx: consumerCtx, stop: stop}, c.Bus.QueueGroup)
	transport.SetSubjects(vertical, lateral)
	return &driver.NetworkSubsystems{Substrate: substrate.NewRedisSubstrate(redisBackend{r}, c.Substrate.KeyPrefix), Bus: transport, Registry: reg, CloseStorage: func() { _ = r.Close() }}, nil
}

type redisBackend struct{ client *redis.Client }

func (r redisBackend) Eval(c context.Context, s string, k []string, a ...interface{}) (interface{}, error) {
	v, err := r.client.Eval(c, s, k, a...).Result()
	if err == redis.Nil {
		return nil, nil
	}
	return v, err
}
func (r redisBackend) Get(c context.Context, k string) (string, error) {
	return r.client.Get(c, k).Result()
}
func (r redisBackend) SetNX(c context.Context, k string, v interface{}, d time.Duration) (bool, error) {
	return r.client.SetNX(c, k, v, d).Result()
}
func (r redisBackend) Del(c context.Context, k ...string) (int64, error) {
	return r.client.Del(c, k...).Result()
}

type etcdBackend struct {
	*clientv3.Client
	rev int64
}

func (e *etcdBackend) GrantLease(c context.Context, ttl int64) (int64, error) {
	r, err := e.Grant(c, ttl)
	if err != nil {
		return 0, err
	}
	return int64(r.ID), nil
}
func (e *etcdBackend) KeepAliveOnce(c context.Context, id int64) error {
	_, err := e.Lease.KeepAliveOnce(c, clientv3.LeaseID(id))
	return err
}
func (e *etcdBackend) RevokeLease(c context.Context, id int64) error {
	_, err := e.Revoke(c, clientv3.LeaseID(id))
	return err
}
func (e *etcdBackend) PutWithLease(c context.Context, k, v string, id int64) error {
	_, err := e.Put(c, k, v, clientv3.WithLease(clientv3.LeaseID(id)))
	return err
}
func (e *etcdBackend) GetPrefix(c context.Context, p string) (map[string]string, error) {
	r, err := e.Get(c, p, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	e.rev = r.Header.Revision
	entries := make(map[string]string, len(r.Kvs))
	for _, kv := range r.Kvs {
		entries[string(kv.Key)] = string(kv.Value)
	}
	return entries, nil
}
func (e *etcdBackend) WatchPrefix(c context.Context, p string, changed func(string, string, string)) error {
	updates := e.Watch(c, p, clientv3.WithPrefix(), clientv3.WithRev(e.rev+1))
	go func() {
		for update := range updates {
			for _, ev := range update.Events {
				changed(ev.Type.String(), string(ev.Kv.Key), string(ev.Kv.Value))
			}
		}
	}()
	return nil
}
