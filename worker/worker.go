package worker

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/zeelrupapara/seo-rank-guardian/config"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/ai"
	natspkg "github.com/zeelrupapara/seo-rank-guardian/pkg/nats"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/scraper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Worker struct {
	Nats        *natspkg.NatsClient
	DB          *gorm.DB
	Log         *zap.SugaredLogger
	Scraper     *scraper.ChainScraper
	AI          ai.AIClient
	Cfg         *config.Config
	subs        []*nats.Subscription
	jobOwnerMu  sync.RWMutex
	jobOwnerMap map[uint]uint // jobID -> userID cache
}

func NewWorker(nc *natspkg.NatsClient, db *gorm.DB, log *zap.SugaredLogger, s *scraper.ChainScraper, aiClient ai.AIClient, cfg *config.Config) *Worker {
	return &Worker{
		Nats:        nc,
		DB:          db,
		Log:         log,
		Scraper:     s,
		AI:          aiClient,
		Cfg:         cfg,
		jobOwnerMap: make(map[uint]uint),
	}
}

func (w *Worker) Listen() error {
	if err := w.Nats.EnsureStream("SRG_JOBS", []string{"srg.jobs.>"}); err != nil {
		return err
	}

	if err := w.Nats.EnsureStream("SRG_LOGS", []string{"srg.logs.>"}); err != nil {
		return err
	}

	if err := w.Nats.EnsureStream("SRG_WS", []string{"srg.ws.>"}); err != nil {
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

func (w *Worker) getJobOwner(jobID uint) (uint, error) {
	w.jobOwnerMu.RLock()
	if uid, ok := w.jobOwnerMap[jobID]; ok {
		w.jobOwnerMu.RUnlock()
		return uid, nil
	}
	w.jobOwnerMu.RUnlock()

	var job model.Job
	if err := w.DB.Unscoped().Select("user_id").Where("id = ?", jobID).First(&job).Error; err != nil {
		return 0, err
	}

	w.jobOwnerMu.Lock()
	w.jobOwnerMap[jobID] = job.UserID
	w.jobOwnerMu.Unlock()
	return job.UserID, nil
}

func (w *Worker) publishEvent(evt model.RunEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		w.Log.Errorf("Failed to marshal event: %v", err)
		return
	}

	logEntry := model.RunEventLog{
		RunID:     evt.RunID,
		JobID:     evt.JobID,
		EventType: evt.Type,
		Data:      data,
	}
	if err := w.DB.Create(&logEntry).Error; err != nil {
		w.Log.Errorf("Failed to persist event: %v", err)
	}

	// Publish raw event to per-run subject (backward compat for REST)
	subject := fmt.Sprintf("srg.logs.%d.%d", evt.JobID, evt.RunID)
	if err := w.Nats.PublishRaw(subject, data); err != nil {
		w.Log.Errorf("Failed to publish event to NATS: %v", err)
	}

	// Publish wrapped event to per-user WS subject (cached lookup)
	userID, err := w.getJobOwner(evt.JobID)
	if err != nil {
		w.Log.Errorf("Failed to look up job owner for WS event: %v", err)
		return
	}

	wsEvent := model.Event{
		Type:    model.EventLogs,
		Payload: evt,
	}
	wsData, err := json.Marshal(wsEvent)
	if err != nil {
		w.Log.Errorf("Failed to marshal WS event: %v", err)
		return
	}

	userSubject := model.SubjectUserEvents(userID)
	if err := w.Nats.PublishRaw(userSubject, wsData); err != nil {
		w.Log.Errorf("Failed to publish WS event to NATS: %v", err)
	}
}

func (w *Worker) Stop() {
	for _, sub := range w.subs {
		sub.Unsubscribe()
	}
	w.Nats.Close()
	w.Log.Info("Worker stopped")
}
