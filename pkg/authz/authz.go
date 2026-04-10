package authz

import (
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	ResourceProfile   = "profile"
	ResourceDashboard = "dashboard"
	ResourceJobs      = "jobs"
	ResourceRuns      = "runs"
	ResourceReports   = "reports"
	ResourceUsers     = "users"
	ResourcePolicies  = "policies"

	ActionRead   = "read"
	ActionWrite  = "write"
	ActionDelete = "delete"
)

func AllResources() []string {
	return []string{ResourceProfile, ResourceDashboard, ResourceJobs, ResourceRuns, ResourceReports, ResourceUsers, ResourcePolicies}
}

func AllActions() []string {
	return []string{ActionRead, ActionWrite, ActionDelete}
}

type Authz struct {
	Enforcer *casbin.CachedEnforcer
	Log      *zap.SugaredLogger
}

func NewAuthz(db *gorm.DB, modelPath string, log *zap.SugaredLogger) (*Authz, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewCachedEnforcer(modelPath, adapter)
	if err != nil {
		return nil, err
	}

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}

	log.Info("Casbin enforcer initialized")

	return &Authz{Enforcer: enforcer, Log: log}, nil
}

func (a *Authz) Enforce(sub, obj, act string) (bool, error) {
	return a.Enforcer.Enforce(sub, obj, act)
}

func (a *Authz) GetAllPolicies() [][]string {
	policies, _ := a.Enforcer.GetPolicy()
	return policies
}

func (a *Authz) AddPolicy(sub, obj, act string) (bool, error) {
	added, err := a.Enforcer.AddPolicy(sub, obj, act)
	if err != nil {
		return false, err
	}
	if added {
		_ = a.Enforcer.SavePolicy()
		a.Enforcer.InvalidateCache()
	}
	return added, nil
}

func (a *Authz) RemovePolicy(sub, obj, act string) (bool, error) {
	removed, err := a.Enforcer.RemovePolicy(sub, obj, act)
	if err != nil {
		return false, err
	}
	if removed {
		_ = a.Enforcer.SavePolicy()
		a.Enforcer.InvalidateCache()
	}
	return removed, nil
}
