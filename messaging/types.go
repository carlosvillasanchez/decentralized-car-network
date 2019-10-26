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
	Request     *[]byte
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
	Message  RumorMessage
	TimeLeft int
	Active   bool
	Mutex    sync.RWMutex
}

func (r *RumormongeringSession) DecrementTimer() {
	r.Mutex.Lock()
	r.TimeLeft--
	r.Mutex.Unlock()
}

func (r *RumormongeringSession) ResetTimer() {
	r.Mutex.Lock()
	r.TimeLeft = 10
	r.Mutex.Unlock()
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

func (a *AtomicRumormongeringSessionMap) GetSession(index string) (session RumormongeringSession) {
	a.Mutex.RLock()
	session = a.RumormongeringSessions[index]
	a.Mutex.RUnlock()
	return
}

func (a *AtomicRumormongeringSessionMap) SetSession(index string, value RumormongeringSession) {
	a.Mutex.Lock()
	a.RumormongeringSessions[index] = value
	a.Mutex.Unlock()
}

// Sets the active value of a session to True if it's not already active.
// Returns whether the active value was set to true or not.
// This method needs to be thread safe since it will start a goroutine that should only run once
func (a *AtomicRumormongeringSessionMap) ActivateSession(index string) bool {
	a.Mutex.Lock()
	session := a.RumormongeringSessions[index]
	if !session.Active {
		session.SetActive(true)
		session.ResetTimer()
		a.RumormongeringSessions[index] = session
		a.Mutex.Unlock()
		return true
	}
	a.Mutex.Unlock()
	return false
}

func (a *AtomicRumormongeringSessionMap) DecrementTimer(index string) {
	a.Mutex.Lock()
	session := a.RumormongeringSessions[index]
	session.DecrementTimer()
	a.RumormongeringSessions[index] = session
	a.Mutex.Unlock()
}

func (a *AtomicRumormongeringSessionMap) DeactivateSession(index string) {
	a.Mutex.Lock()
	session := a.RumormongeringSessions[index]
	session.SetActive(false)
	a.RumormongeringSessions[index] = session
	a.Mutex.Unlock()
}

func (a *AtomicRumormongeringSessionMap) ResetTimer(index string) {
	a.Mutex.Lock()
	session := a.RumormongeringSessions[index]
	session.ResetTimer()
	a.RumormongeringSessions[index] = session
	a.Mutex.Unlock()
}
