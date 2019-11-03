package main

import (
	"flag"
	"github.com/tormey97/Peerster/gossiper"
	"github.com/tormey97/Peerster/messaging"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type Origin int

const (
	Client Origin = iota
	Server
)

func createPeerster() gossiper.Peerster {
	UIPort := flag.String("UIPort", "8080", "the port the client uses to communicate with peerster")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "the address of the peerster")
	name := flag.String("name", "nodeA", "the Name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("simple", false, "Simple mode")
	antiEntropy := flag.Int("antiEntropy", 10, "Anti entropy timer")
	rTimer := flag.Int("rtimer", 0, "Route rumor message interval timer")
	flag.Parse()
	peersList := []string{}
	if *peers != "" {
		peersList = strings.Split(*peers, ",")
	}
	return gossiper.Peerster{
		UIPort:           *UIPort,
		GossipAddress:    *gossipAddr,
		KnownPeers:       peersList,
		Name:             *name,
		Simple:           *simple,
		AntiEntropyTimer: *antiEntropy,
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
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
func main() {
	peerster := createPeerster()
	go peerster.Listen(gossiper.Server)
	go peerster.ListenFrontend()
	peerster.AntiEntropy()
	peerster.SendRouteMessages()
	peerster.Listen(gossiper.Client)
}
