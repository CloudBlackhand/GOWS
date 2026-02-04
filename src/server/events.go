package server

import (
	"encoding/json"
	"fmt"
	"github.com/devlikeapro/gows/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"reflect"
	"strings"
)

// eventPayload carries pre-marshalled event to listeners so we marshal once per event and avoid holding large structs in channel buffers.
type eventPayload struct {
	Event string
	Data  string
}

func (s *Server) safeMarshal(v interface{}) (result string) {
	defer func() {
		if err := recover(); err != nil {
			// Print log error and ignore
			s.log.Errorf("Panic happened when marshaling data: %v", err)
			result = ""
		}
	}()
	data, err := json.Marshal(v)
	if err != nil {
		s.log.Errorf("Error when marshaling data: %v", err)
		return ""
	}
	result = string(data)
	return result
}

func (s *Server) StreamEvents(req *__.Session, stream grpc.ServerStreamingServer[__.EventJson]) error {
	name := req.GetId()
	streamId := uuid.New()
	listener := s.addListener(name, streamId)
	defer s.removeListener(name, streamId)
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case payload := <-listener:
			data := __.EventJson{
				Session: name,
				Event:   payload.Event,
				Data:    payload.Data,
			}
			err := stream.Send(&data)
			if err != nil {
				return err
			}
		}
	}
}

func (s *Server) IssueEvent(session string, event interface{}) {
	jsonString := s.safeMarshal(event)
	if jsonString == "" {
		return
	}
	eventType := reflect.TypeOf(event).String()
	eventType = strings.TrimPrefix(eventType, "*")
	payload := eventPayload{Event: eventType, Data: jsonString}

	listeners := s.getListeners(session)
	// Send to listeners in blocking loop instead of creating goroutine per listener to reduce RAM.
	// This reduces goroutine creation overhead and stack memory during event spikes.
	for _, listener := range listeners {
		func(ch chan eventPayload) {
			defer func() {
				if err := recover(); err != nil {
					fmt.Print("Error when sending event to listener: ", err)
				}
			}()
			// Non-blocking send: if channel is full, skip to avoid blocking other listeners.
			select {
			case ch <- payload:
			default:
				// Channel full, skip this listener to avoid blocking
			}
		}(listener)
	}
}

func (s *Server) addListener(session string, id uuid.UUID) chan eventPayload {
	s.listenersLock.Lock()
	defer s.listenersLock.Unlock()

	listener := make(chan eventPayload, 1) // minimal buffer to reduce RAM; events may be dropped if consumer is slow
	sessionListeners, ok := s.listeners[session]
	if !ok {
		sessionListeners = map[uuid.UUID]chan eventPayload{}
		s.listeners[session] = sessionListeners
	}
	sessionListeners[id] = listener
	return listener
}

func (s *Server) removeListener(session string, id uuid.UUID) {
	s.listenersLock.Lock()
	defer s.listenersLock.Unlock()
	listener, ok := s.listeners[session][id]
	if !ok {
		return
	}
	delete(s.listeners[session], id)
	// if it's the last listener, remove the session
	if len(s.listeners[session]) == 0 {
		delete(s.listeners, session)
	}
	close(listener)
}

func (s *Server) getListeners(session string) []chan eventPayload {
	s.listenersLock.RLock()
	defer s.listenersLock.RUnlock()
	sessionListeners := s.listeners[session]
	listeners := make([]chan eventPayload, 0, len(sessionListeners))
	for _, listener := range sessionListeners {
		listeners = append(listeners, listener)
	}
	return listeners
}
