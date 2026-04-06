package scheduler

import (
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TriggerFunc is the shared scan trigger injected from the HTTP layer.
type TriggerFunc func(job model.Job, triggeredBy string, userID uint) error

type Scheduler struct {
	cron      *cron.Cron
	entries   map[uint]cron.EntryID // jobID → entryID
	mu        sync.Mutex
	db        *gorm.DB
	log       *zap.SugaredLogger
	triggerFn TriggerFunc
}

func New(db *gorm.DB, log *zap.SugaredLogger, fn TriggerFunc) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		entries:   make(map[uint]cron.EntryID),
		db:        db,
		log:       log,
		triggerFn: fn,
	}
}

// LoadAll registers cron entries for all active jobs with a schedule on startup.
func (s *Scheduler) LoadAll() error {
	var jobs []model.Job
	if err := s.db.Where("is_active = true AND schedule_time != ''").Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		s.AddJob(job)
	}
	s.log.Infof("Scheduler: loaded %d scheduled jobs", len(jobs))
	return nil
}

// AddJob registers (or replaces) a cron entry for a job.
func (s *Scheduler) AddJob(job model.Job) {
	if job.ScheduleTime == "" {
		s.RemoveJob(job.ID)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[job.ID]; ok {
		s.cron.Remove(id)
	}
	jobID := job.ID
	id, err := s.cron.AddFunc(job.ScheduleTime, func() {
		// Reload job fresh from DB before firing
		var fresh model.Job
		if err := s.db.Preload("Keywords").First(&fresh, jobID).Error; err != nil {
			s.log.Errorf("Scheduler: failed to reload job %d: %v", jobID, err)
			return
		}
		if !fresh.IsActive || fresh.ScheduleTime == "" {
			s.RemoveJob(fresh.ID)
			return
		}
		if err := s.triggerFn(fresh, "scheduled", fresh.UserID); err != nil {
			s.log.Warnf("Scheduler: trigger for job %d skipped: %v", jobID, err)
		}
	})
	if err != nil {
		s.log.Errorf("Scheduler: invalid cron expression for job %d (%s): %v", job.ID, job.ScheduleTime, err)
		return
	}
	s.entries[job.ID] = id
	s.log.Infof("Scheduler: registered job %d with schedule '%s'", job.ID, job.ScheduleTime)
}

// RemoveJob removes a cron entry for a job.
func (s *Scheduler) RemoveJob(jobID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[jobID]; ok {
		s.cron.Remove(id)
		delete(s.entries, jobID)
	}
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop()  { s.cron.Stop() }
