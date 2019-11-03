package messaging

import (
	"sync"
)

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type GossipPacket struct {
	Simple      *SimpleMessage
	Rumor       *RumorMessage
	Status      *StatusPacket
	Private     *PrivateMessage
	DataRequest *DataRequest
	DataReply   *DataReply
}

type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type Message struct {
	Text        string
	Destination *string
	Request     string
	File        string
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

type RumormongeringSession struct {
	Message RumorMessage
	Active  bool
	Channel chan bool
	Mutex   sync.RWMutex
}

func (r *RumormongeringSession) ResetTimer() {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()
	if r.Active {
		r.Channel <- false // False means "don't interrupt"
	}
}

func (r *RumormongeringSession) SetActive(value bool) {
	r.Mutex.Lock()
	r.Active = value
	r.Mutex.Unlock()
}

type AtomicRumormongeringSessionMap struct {
	RumormongeringSessions map[string]RumormongeringSession
	Mutex                  sync.RWMutex
}

func (a *AtomicRumormongeringSessionMap) GetSession(index string) (session RumormongeringSession, ok bool) {
	a.Mutex.RLock()
	defer a.Mutex.RUnlock()
	session, ok = a.RumormongeringSessions[index]
	return
}

func (a *AtomicRumormongeringSessionMap) SetSession(index string, value RumormongeringSession) {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	a.RumormongeringSessions[index] = value
}

// Sets the active value of a session to True if it's not already active.
// Returns whether the active value was set to true or not.
// This method needs to be thread safe since it will start a goroutine that should only run once
func (a *AtomicRumormongeringSessionMap) ActivateSession(index string) bool {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	session, ok := a.RumormongeringSessions[index]
	if !ok {
		return false
	}
	if !session.Active {
		session.SetActive(true)
		a.RumormongeringSessions[index] = session
		return true
	}
	return false
}

func (a *AtomicRumormongeringSessionMap) InterruptSession(index string) {
	a.Mutex.RLock()
	defer a.Mutex.RUnlock()
	if a.RumormongeringSessions[index].Active {
		a.RumormongeringSessions[index].Channel <- true
	}
}

func (a *AtomicRumormongeringSessionMap) DeactivateSession(index string) {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	session, ok := a.RumormongeringSessions[index]
	if !ok {
		return
	}
	session.SetActive(false)
	a.RumormongeringSessions[index] = session
}

func (a *AtomicRumormongeringSessionMap) ResetTimer(index string) {
	a.Mutex.Lock()
	defer a.Mutex.Unlock()
	session, ok := a.RumormongeringSessions[index]
	if !ok {
		return
	}
	session.ResetTimer()
	a.RumormongeringSessions[index] = session

}
