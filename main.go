package main

import (
	"flag"
	"fmt"
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
	fmt.Println(peersList, *peers, "???")
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
		ReceivedMessages:        map[string][]messaging.RumorMessage{},
		ReceivedPrivateMessages: map[string][]messaging.PrivateMessage{},
		MsgSeqNumber:            1,
		Want:                    []messaging.PeerStatus{},
		Conn:                    net.UDPConn{},
		RTimer:                  *rTimer,
		NextHopTable:            map[string]string{},
		SharedFiles:             map[string]gossiper.SharedFile{},
		FileChunks:              map[string][]byte{},
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
	fmt.Println(peerster.String())
	addr := messaging.StringAddrToUDPAddr(peerster.GossipAddress)
	fmt.Println(addr.String(), string(addr.IP), peerster.GossipAddress)
	go peerster.Listen(gossiper.Server)
	go peerster.ListenFrontend()
	peerster.AntiEntropy()
	peerster.SendRouteMessage()
	peerster.SendRouteMessages()
	peerster.Listen(gossiper.Client)
}
