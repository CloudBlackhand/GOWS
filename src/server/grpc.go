package server

import (
	"github.com/devlikeapro/gows/gows"
	gowsLog "github.com/devlikeapro/gows/log"
	pb "github.com/devlikeapro/gows/proto"
	"github.com/google/uuid"
	waLog "go.mau.fi/whatsmeow/util/log"
	"sync"
)

// assert that Server implements pb.MessageServiceServer
var _ pb.MessageServiceServer = (*Server)(nil)

// assert that Server implements pb.EventStreamServer
var _ pb.EventStreamServer = (*Server)(nil)

type Server struct {
	pb.UnsafeMessageServiceServer
	pb.UnsafeEventStreamServer
	Sm  *gows.SessionManager
	log waLog.Logger

	// session id -> listener id -> event channel (eventPayload = pre-marshalled to limit RAM)
	listeners     map[string]map[uuid.UUID]chan eventPayload
	listenersLock sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		Sm:            gows.NewSessionManager(),
		log:           gowsLog.Stdout("gRPC", "INFO", false),
		listeners:     map[string]map[uuid.UUID]chan eventPayload{},
		listenersLock: sync.RWMutex{},
	}
}
