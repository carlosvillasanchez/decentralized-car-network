package carDecentralized

import (
	//"flag"
	"fmt"
	"math/rand"
	"net"
	"strconv"
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

func createPeerster(gossipAddr *string, mapString *string, name *string, peers *string, antiEntropy *int, rTimer *int, startPosition *string, endPosition *string, areaPeers *string) gossiper.Peerster {

	UIPort := "8080"
	fmt.Println("STARTING!!")
	simple := false

	peersList := []string{}
	if *peers != "" {
		peersList = strings.Split(*peers, ",")
	}
	areaPeersList := []string{}
	if *areaPeers != "" {
		areaPeersList = strings.Split(*areaPeers, ",")

	}
	fmt.Println("prueba", areaPeersList)
	// Creation of the map, if empty put the empty map
	var carMap [10][10]utils.Square
	if *mapString == "" {
		carMap = utils.StringToCarMap(utils.ArrayStringToString(emptyMap))
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
	fmt.Println("CARPATH", carPath, finalCarMap, startPositionP, endPositionP)

	peerster := gossiper.Peerster{
		UIPort:           UIPort,
		GossipAddress:    *gossipAddr,
		KnownPeers:       peersList,
		Name:             *name,
		Simple:           simple,
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
	for _, v := range areaPeersList {
		peerster.SaveCarInAreaStructure("", utils.Position{}, v)
	}

	return peerster
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func Start(gossipAddr *string, mapString *string, name *string, peers *string, antiEntropy *int, rTimer *int, startPosition *string, endPosition *string, areaPeers *string, parkingSearch *bool) {
	peerster := createPeerster(gossipAddr, mapString, name, peers, antiEntropy, rTimer, startPosition, endPosition, areaPeers)
	fmt.Println(*name)
	for _, value := range peerster.PosCarsInArea.Slice {
		fmt.Printf("%+v\n", value)
	}
	peerster.SubscribeToNewsgroup(strconv.Itoa(utils.AreaPositioner(peerster.PathCar[0])))
	if *parkingSearch {
		peerster.SubscribeToNewsgroup(gossiper.ParkingNewsGroup)
	}
	// go peerster.ListenFrontend()
	peerster.AntiEntropy()
	peerster.SendRouteMessages()

	//Broadcast the car position in the current area of the car
	go func() {
		for {
			time.Sleep(time.Duration(peerster.BroadcastTimer) * time.Second)
			peerster.BroadcastCarPosition()
		}
	}()

	//Rutine that sends information to the server
	peerster.SendInfoToServer()
	// peerster.Listen(gossiper.Client)

	//Moves the car in the map
	peerster.MoveCarPosition()

	//Handles all the incoming messages and the responses
	peerster.Listen(gossiper.Server)
}
