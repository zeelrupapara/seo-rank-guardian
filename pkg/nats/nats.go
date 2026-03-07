package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

type NatsClient struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
	Log  *zap.SugaredLogger
}

func NewNatsClient(cfg config.NATSConfig, log *zap.SugaredLogger) (*NatsClient, error) {
	nc, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	log.Info("Connected to NATS JetStream")

	return &NatsClient{Conn: nc, JS: js, Log: log}, nil
}

func (n *NatsClient) EnsureStream(name string, subjects []string) error {
	_, err := n.JS.AddStream(&nats.StreamConfig{
		Name:     name,
		Subjects: subjects,
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", name, err)
	}
	n.Log.Infof("Stream %s ensured", name)
	return nil
}

func (n *NatsClient) Publish(subject string, data []byte) error {
	_, err := n.JS.Publish(subject, data)
	return err
}

func (n *NatsClient) Subscribe(subject, durable string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return n.JS.Subscribe(subject, handler, nats.Durable(durable), nats.DeliverAll())
}

func (n *NatsClient) Close() {
	n.Conn.Close()
}
