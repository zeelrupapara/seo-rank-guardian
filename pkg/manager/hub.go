package manager

import (
	"sync"

	"github.com/zeelrupapara/seo-rank-guardian/model"
	"go.uber.org/zap"
)

type Handler func(*Ctx) error

type RouterMap map[model.EventType]Handler

type Hub struct {
	clients   map[uint]map[*Client]struct{} // userID -> set of clients
	routerMap RouterMap
	mu        sync.RWMutex
	Log       *zap.SugaredLogger
}

func NewHub(log *zap.SugaredLogger) *Hub {
	return &Hub{
		clients:   make(map[uint]map[*Client]struct{}),
		routerMap: make(RouterMap),
		Log:       log,
	}
}

func (h *Hub) GetAll(userID uint) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clientSet, ok := h.clients[userID]
	if !ok {
		return nil
	}
	result := make([]*Client, 0, len(clientSet))
	for c := range clientSet {
		result = append(result, c)
	}
	return result
}

func (h *Hub) Store(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.UserID] == nil {
		h.clients[client.UserID] = make(map[*Client]struct{})
	}
	h.clients[client.UserID][client] = struct{}{}
	h.Log.Infof("Client stored for user %d (total: %d)", client.UserID, len(h.clients[client.UserID]))
}

func (h *Hub) Delete(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clientSet, ok := h.clients[client.UserID]; ok {
		delete(clientSet, client)
		if len(clientSet) == 0 {
			delete(h.clients, client.UserID)
		}
	}
	h.Log.Infof("Client removed for user %d", client.UserID)
}

func (h *Hub) RegisterRoute(event model.EventType, handler Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.routerMap[event] = handler
}

func (h *Hub) getHandler(event model.EventType) (Handler, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	handler, ok := h.routerMap[event]
	return handler, ok
}
