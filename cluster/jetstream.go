package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"strings"
	"sync"
	"time"
)

const streamName = "OCTPEX_TASKS"
const replyKey = "Octpex-Reply"
const deadlineKey = "Octpex-Deadline"

func ensureStream(js nats.JetStreamContext, subjects []string) error {
	stream, err := js.StreamInfo(streamName)
	if errors.Is(err, nats.ErrStreamNotFound) {
		stream, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: subjects, Storage: nats.FileStorage, MaxAge: 5 * time.Minute})
		if err != nil {
			stream, err = js.StreamInfo(streamName)
		}
	}
	if err != nil {
		return err
	}
	for _, subject := range subjects {
		found := false
		for _, existing := range stream.Config.Subjects {
			if existing == subject {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("stream %s missing subject %s", streamName, subject)
		}
	}
	return nil
}

type jetBus struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	ctx  context.Context
	stop context.CancelFunc
	wg   sync.WaitGroup
}
type reply struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

func (b *jetBus) PublishMsg(ctx context.Context, subject string, data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	inbox := nats.NewInbox()
	sub, err := b.nc.SubscribeSync(inbox)
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()
	msg := nats.NewMsg(subject)
	msg.Data = data
	msg.Header.Set(replyKey, inbox)
	msg.Header.Set(deadlineKey, deadline.UTC().Format(time.RFC3339Nano))
	if _, err = b.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return nil, err
	}
	response, err := sub.NextMsgWithContext(ctx)
	if err != nil {
		return nil, err
	}
	var out reply
	if err = json.Unmarshal(response.Data, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, errors.New(out.Error)
	}
	return out.Data, nil
}
func (b *jetBus) SubscribeQueue(subject, group string, handle func(context.Context, []byte) ([]byte, error)) error {
	id := sha256.Sum256([]byte(group + "/" + subject))
	durable := fmt.Sprintf("arm_%x", id[:12])
	sub, err := b.js.PullSubscribe(subject, durable, nats.BindStream(streamName))
	if err != nil {
		return err
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for b.ctx.Err() == nil {
			msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
			if err != nil {
				if !errors.Is(err, nats.ErrTimeout) {
					select {
					case <-b.ctx.Done():
						return
					case <-time.After(200 * time.Millisecond):
					}
				}
				continue
			}
			for _, msg := range msgs {
				dest := msg.Header.Get(replyKey)
				expires, err := time.Parse(time.RFC3339Nano, msg.Header.Get(deadlineKey))
				if !strings.HasPrefix(dest, "_INBOX.") || strings.ContainsAny(dest, "* >\t\r\n") || err != nil || !expires.After(time.Now()) {
					_ = msg.Term()
					continue
				}
				ctx, cancel := context.WithDeadline(b.ctx, expires)
				result, err := handle(ctx, msg.Data)
				cancel()
				response := reply{Data: result}
				if err != nil {
					response = reply{Error: err.Error()}
				}
				data, err := json.Marshal(response)
				if err != nil {
					_ = msg.Term()
					continue
				}
				if err = b.nc.Publish(dest, data); err == nil {
					_ = msg.Ack()
				}
			}
		}
	}()
	return nil
}
func (b *jetBus) Close() error { b.stop(); b.wg.Wait(); b.nc.Close(); return nil }
