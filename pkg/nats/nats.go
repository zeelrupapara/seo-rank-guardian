package nats

import (
	"fmt"
	"time"

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
	info, err := n.JS.StreamInfo(name)
	if err == nil && info != nil {
		// Stream exists — update if needed
		_, err = n.JS.UpdateStream(&nats.StreamConfig{
			Name:     name,
			Subjects: subjects,
			Storage:  nats.FileStorage,
		})
		if err != nil {
			n.Log.Warnf("Failed to update stream %s (using existing): %v", name, err)
		}
	} else {
		// Create new stream
		_, err = n.JS.AddStream(&nats.StreamConfig{
			Name:     name,
			Subjects: subjects,
			Storage:  nats.FileStorage,
		})
		if err != nil {
			return fmt.Errorf("failed to create stream %s: %w", name, err)
		}
	}
	n.Log.Infof("Stream %s ensured", name)
	return nil
}

func (n *NatsClient) Publish(subject string, data []byte) error {
	_, err := n.JS.Publish(subject, data)
	return err
}

func (n *NatsClient) Subscribe(subject, durable string, handler nats.MsgHandler) (*nats.Subscription, error) {
	opts := []nats.SubOpt{
		nats.Durable(durable),
		nats.DeliverAll(),
		nats.MaxDeliver(3),
		nats.AckWait(5 * time.Minute),
		nats.BackOff([]time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}),
	}

	sub, err := n.JS.Subscribe(subject, handler, opts...)
	if err != nil {
		// Consumer config may have changed — delete and recreate
		n.Log.Warnf("Subscribe failed for %s/%s, attempting consumer recreate: %v", subject, durable, err)
		// Try to find which stream this subject belongs to
		for _, streamName := range []string{"SRG_JOBS", "SRG_LOGS", "SRG_WS"} {
			info, infoErr := n.JS.ConsumerInfo(streamName, durable)
			if infoErr == nil && info != nil {
				_ = n.JS.DeleteConsumer(streamName, durable)
				n.Log.Infof("Deleted stale consumer %s from stream %s, recreating", durable, streamName)
				break
			}
		}
		sub, err = n.JS.Subscribe(subject, handler, opts...)
	}
	return sub, err
}

func (n *NatsClient) PublishRaw(subject string, data []byte) error {
	return n.Conn.Publish(subject, data)
}

func (n *NatsClient) SubscribeRaw(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return n.Conn.Subscribe(subject, handler)
}

func (n *NatsClient) Close() {
	n.Conn.Close()
}
