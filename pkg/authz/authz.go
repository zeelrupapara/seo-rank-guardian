package authz

import (
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	ResourceUsers   = "users"
	ResourceProfile = "profile"

	ActionRead   = "read"
	ActionWrite  = "write"
	ActionDelete = "delete"
)

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
