package messaging

import (
	"sync"

	"github.com/tormey97/decentralized-car-network/utils"
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
	*AreaChangeResponse
	*AlertPolice
}

type GossipPacket struct {
	Simple        *SimpleMessage
	Area          *AreaMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
	Colision      *ColisionResolution
	ServerAlert   *utils.ServerMessage
}

type AreaChangeResponse struct {
}
type AlertPolice struct {
	utils.Position
	Origin   string
	Evidence []byte
}

type AreaChangeMessage struct {
	NextPosition    utils.Position
	CurrentPosition utils.Position
}

type AccidentMessage struct {
	utils.Position
}

type RumorMessage struct {
	Origin    string
	ID        uint32
	Text      string
	Newsgroup string
	*AreaChangeMessage
	*AccidentMessage
}

type Message struct {
	Text        string
	Destination *string
	Request     string
	File        string
	Budget      int
	Keywords    []string
}

type AreaMessage struct {
	Origin string // Name of the car
	// ID       uint32 // ID of the message, cars only analyze the message with the highest ID
	Position utils.Position
}
type FreeSpotMessage struct {
	Origin          string // Name of the Announcing car
	ID              uint32
	ParkingPosition utils.Position // Position of the parking spot
	Taken           bool           // Wheter the spot is occuped already or not
}
type ColisionResolution struct {
	Origin     string
	CoinResult int
}

// type Position struct {
// 	X int // X position of the car
// 	Y int // Y position of the car
// }
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

// type Square struct { // One grid/square in the map
// 	Type string // Type of the square, parking spot, accident, normal etc
// }

// type SimulatedMap struct { // The entire simulated map
// 	sync.RWMutex
// 	Grid [9][9]Square // Matrix representing the whole map
// }
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

type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}
type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}
type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
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
