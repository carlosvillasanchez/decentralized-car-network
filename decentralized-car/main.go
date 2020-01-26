package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tormey97/decentralized-car-network/decentralized-car/gossiper"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
)

type Origin int

const (
	Client Origin = iota
	Server
)

var emptyMap = [][]string{
	{"B", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "B", "N"},
	{"N", "N", "N", "N", "N", "N", "N", "N", "N"},
}

func createPeerster() gossiper.Peerster {
	UIPort := flag.String("UIPort", "8080", "the port the client uses to communicate with peerster")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "the address of the peerster")
	mapString := flag.String("map", "", "Matrix representing the map in string representation")
	name := flag.String("name", "nodeA", "the Name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("simple", false, "Simple mode")
	antiEntropy := flag.Int("antiEntropy", 10, "Anti entropy timer")
	rTimer := flag.Int("rtimer", 0, "Route rumor message interval timer")
	startPosition := flag.String("startP", "0,0", "the starting position of the car")   // 5,9 x=5, y = 9
	endPosition := flag.String("endP", "8,8", "the ending desired position of the car") // 5,9 x=5, y = 9
	flag.Parse()
	peersList := []string{}
	if *peers != "" {
		peersList = strings.Split(*peers, ",")
	}

	// Creation of the map, if empty put the empty map
	var carMap [9][9]utils.Square
	if *mapString == "" {
		carMap = utils.StringToCarMap(utils.ArrayStringToString(emptyMap))
		fmt.Println("fernando is a dumbass", utils.ArrayStringToString(emptyMap))
	} else {
		carMap = utils.StringToCarMap(*mapString)
	}
	var finalCarMap utils.SimulatedMap
	finalCarMap.Grid = carMap
	//Assigment of the positions of the car
	startPositionP := utils.StringToPosition(*startPosition)
	endPositionP := utils.StringToPosition(*endPosition)

	// Creation of the path of the car, except if is a police car
	var carPath []utils.Position
	if (startPositionP.X != -1) && (startPositionP.Y != -1) {
		carPath = gossiper.CreatePath(&finalCarMap, startPositionP, endPositionP, []utils.Position{})
	}

	peerster := gossiper.Peerster{
		UIPort:           *UIPort,
		GossipAddress:    *gossipAddr,
		KnownPeers:       peersList,
		Name:             *name,
		Simple:           *simple,
		AntiEntropyTimer: *antiEntropy,
		CarMap:           &finalCarMap,
		PathCar:          carPath,
		BroadcastTimer:   gossiper.BroadcastTimer,
		PosCarsInArea: utils.CarInfomartionList{
			Slice: make([]*utils.CarInformation, 0),
			Mutex: sync.RWMutex{},
		},
		RumormongeringSessions: messaging.AtomicRumormongeringSessionMap{
			RumormongeringSessions: map[string]messaging.RumormongeringSession{},
			Mutex:                  sync.RWMutex{},
		},
		ReceivedMessages: struct {
			Map   map[string][]messaging.RumorMessage
			Mutex sync.RWMutex
		}{Map: map[string][]messaging.RumorMessage{}, Mutex: sync.RWMutex{}},
		ReceivedPrivateMessages: struct {
			Map   map[string][]messaging.PrivateMessage
			Mutex sync.RWMutex
		}{Map: map[string][]messaging.PrivateMessage{}, Mutex: sync.RWMutex{}},
		MsgSeqNumber: 1,
		Want:         []messaging.PeerStatus{},
		Conn:         net.UDPConn{},
		RTimer:       *rTimer,
		NextHopTable: struct {
			Map   map[string]string
			Mutex sync.RWMutex
		}{Map: map[string]string{}, Mutex: sync.RWMutex{}},
		SharedFiles: struct {
			Map   map[string]gossiper.SharedFile
			Mutex sync.RWMutex
		}{Map: map[string]gossiper.SharedFile{}, Mutex: sync.RWMutex{}},
		FileChunks: struct {
			Map   map[string][]byte
			Mutex sync.RWMutex
		}{Map: map[string][]byte{}, Mutex: sync.RWMutex{}},
		DownloadingFiles: gossiper.DownloadingFiles{
			Map:   map[string]gossiper.FileBeingDownloaded{},
			Mutex: sync.RWMutex{},
		},
	}
	for _, v := range peersList {
		peerster.SaveCarInAreaStructure("", utils.Position{}, v)
	}

	return peerster
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
func main() {
	peerster := createPeerster()

	// go peerster.ListenFrontend()
	peerster.AntiEntropy()
	peerster.SendRouteMessages()

	//Broadcast the car position in the current area of the car
	peerster.BroadcastCarPosition()

	//Rutine that sends information to the server
	peerster.SendInfoToServer()
	// peerster.Listen(gossiper.Client)

	//Moves the car in the map
	peerster.MoveCarPosition()

	//Handles all the incoming messages and the responses
	peerster.Listen(gossiper.Server)
}
