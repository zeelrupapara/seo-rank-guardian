package worker

import (
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nats-io/nats.go"
)

type Worker struct {
	Nats *natspkg.NatsClient
	DB   *gorm.DB
	Log  *zap.SugaredLogger
	subs []*nats.Subscription
}

func NewWorker(nc *natspkg.NatsClient, db *gorm.DB, log *zap.SugaredLogger) *Worker {
	return &Worker{
		Nats: nc,
		DB:   db,
		Log:  log,
	}
}

func (w *Worker) Listen() error {
	if err := w.Nats.EnsureStream("SRG_JOBS", []string{"srg.jobs.>"}); err != nil {
		return err
	}

	sub, err := w.Nats.Subscribe("srg.jobs.>", "srg-worker", func(msg *nats.Msg) {
		w.Log.Infof("Received job on subject: %s", msg.Subject)
		if err := HandleJob(w, msg.Subject, msg.Data); err != nil {
			w.Log.Errorf("Job handler error: %v", err)
			msg.Nak()
			return
		}
		msg.Ack()
	})
	if err != nil {
		return err
	}

	w.subs = append(w.subs, sub)
	w.Log.Info("Worker listening for jobs on srg.jobs.>")
	return nil
}

func (w *Worker) Stop() {
	for _, sub := range w.subs {
		sub.Unsubscribe()
	}
	w.Nats.Close()
	w.Log.Info("Worker stopped")
}
