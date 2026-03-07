package smtp

import (
	"fmt"

	"github.com/zeelrupapara/seo-rank-guardian/config"
	"go.uber.org/zap"
)

type SMTPClient struct {
	cfg config.SMTPConfig
	log *zap.SugaredLogger
}

func NewSMTPClient(cfg config.SMTPConfig, log *zap.SugaredLogger) *SMTPClient {
	return &SMTPClient{cfg: cfg, log: log}
}

func (s *SMTPClient) SendEmail(to, subject, body string) error {
	s.log.Infof("SendEmail stub: to=%s subject=%s", to, subject)
	return fmt.Errorf("SMTP not yet implemented")
}
