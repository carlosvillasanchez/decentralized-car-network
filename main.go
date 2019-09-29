package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/tormey97/Peerster/messaging"
	"log"
	"net"
	"strings"
)

type Origin int

const (
	Client Origin = iota
	Server
)

type Peerster struct {
	UIPort     string
	gossipAddr string
	knownPeers []string
	name       string
	simple     bool
}

func (peerster Peerster) String() string {
	return fmt.Sprintf(
		"UIPort: %s, gossipAddr: %s, knownPeers: %s, name: %s, simple: %s", peerster.UIPort, peerster.gossipAddr, peerster.knownPeers, peerster.name, peerster.simple)
}

func (peerster Peerster) listen(origin Origin) {
	var addr string
	switch origin {
	case Client:
		addr = "127.0.0.1:" + peerster.UIPort
	case Server:
		addr = peerster.gossipAddr
	}
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Error: could not listen. Origin: %s, error: %s", origin, err)
	}
	for {
		buffer := make([]byte, 1024)
		conn.ReadFrom(buffer)
		var packet messaging.GossipPacket
		switch origin {
		case Client:
			fmt.Println("CLIENT MESSAGE " + string(buffer))
			packet = messaging.GossipPacket{Simple: peerster.createMessage(string(buffer))}
			peerster.sendToKnownPeers(packet)
		case Server:
			fmt.Println("SERVER MESSAGE" + string(buffer))
		}
	}
}

func (peerster Peerster) createMessage(msg string) *messaging.SimpleMessage {
	return &messaging.SimpleMessage{
		OriginalName:  peerster.name,
		RelayPeerAddr: peerster.gossipAddr,
		Contents:      msg,
	}
}

// Sends a GossipPacket to all known peers
func (peerster Peerster) sendToKnownPeers(packet messaging.GossipPacket) error {
	for i := range peerster.knownPeers {
		peer := peerster.knownPeers[i]
		conn, err := net.Dial("udp", peer)
		if err != nil {
			return err
		}
		packetBytes, err := protobuf.Encode(packet)
		if err != nil {
			return err
		}
		conn.Write(packetBytes)
	}
	return nil
}

func createPeerster() Peerster {
	UIPort := flag.String("UIPort", "8080", "the port the client uses to communicate with peerster")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "the address of the peerster")
	name := flag.String("name", "nodeA", "the name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("simple", true, "simple mode")
	flag.Parse()
	return Peerster{
		UIPort:     *UIPort,
		gossipAddr: *gossipAddr,
		knownPeers: strings.Split(*peers, ","),
		name:       *name,
		simple:     *simple,
	}
}

func main() {
	peerster := createPeerster()
	fmt.Println(peerster.String())
	peerster.listen(Client)

}
