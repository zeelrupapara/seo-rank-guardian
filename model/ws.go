package model

import "fmt"

type EventType string

type MessageType int32

const (
	JsonMessage MessageType = 0
	PingMessage MessageType = 3
	PongMessage MessageType = 4
)

const (
	EventLogs EventType = "logs"
)

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
	Format  MessageType `json:"-"`
}

var SubjectUserEvents = func(userId uint) string {
	return fmt.Sprintf("srg.ws.%d", userId)
}
