package manager

import (
	"fmt"

	"github.com/zeelrupapara/seo-rank-guardian/model"
)

type Ctx struct {
	Client *Client
	Type   model.EventType
	Data   []byte
	Event  *model.Event
}

func (c *Ctx) SendEvent(e *model.Event) error {
	select {
	case c.Client.Egress <- e:
		return nil
	default:
		return fmt.Errorf("egress channel full for user %d", c.Client.UserID)
	}
}
