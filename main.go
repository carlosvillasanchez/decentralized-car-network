package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/tormey97/Peerster/messaging"
	"log"
	"net"
	"strconv"
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
	want       []messaging.PeerStatus
}

func (peerster Peerster) String() string {
	return fmt.Sprintf(
		"UIPort: %s, gossipAddr: %s, knownPeers: %s, name: %s, simple: %t", peerster.UIPort, peerster.gossipAddr, peerster.knownPeers, peerster.name, peerster.simple)
}

func (peerster Peerster) createConnection(origin Origin) (net.UDPConn, error) {
	var addr string
	switch origin {
	case Client:
		addr = "127.0.0.1:" + peerster.UIPort
	case Server:
		addr = peerster.gossipAddr
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return net.UDPConn{}, err
	}
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return net.UDPConn{}, err
	}
	return *udpConn, nil
}

func readFromConnection(conn net.UDPConn) ([]byte, *net.UDPAddr, error) {
	buffer := make([]byte, 1024)
	n, originAddr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, nil, err
	}
	buffer = buffer[:n]
	return buffer, originAddr, nil
}

func (peerster Peerster) clientReceive(buffer []byte, packet messaging.GossipPacket) {
	fmt.Println("CLIENT MESSAGE " + string(buffer))
	packet = messaging.GossipPacket{Simple: peerster.createSimpleMessage(string(buffer))}
	err := peerster.sendToKnownPeers(packet, []string{})
	if err != nil {
		fmt.Printf("Error: could not send packet from client, reason: %s", err)
	}
}

func (peerster Peerster) serverReceive(buffer []byte, originAddr net.UDPAddr, packet messaging.GossipPacket) {
	receivedPacket := &messaging.GossipPacket{}
	err := protobuf.Decode(buffer, receivedPacket)
	if err != nil {
		fmt.Printf("Error: could not decode packet, reason: %s", err)
	}
	//TODO Handle SimpleMessage and Rumor cases differently. If it's a simplemessage, the relay origin addr is probably inside the message
	fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s \n", receivedPacket.Simple.OriginalName, receivedPacket.Simple.RelayPeerAddr, receivedPacket.Simple.Contents)
	addr := originAddr.IP.String() + ":" + strconv.Itoa(originAddr.Port)
	if receivedPacket.Simple.RelayPeerAddr != "" {
		addr = receivedPacket.Simple.RelayPeerAddr
	}
	blacklist := []string{addr} // we won't send a message to these peers
	peerster.addToKnownPeers(addr)
	receivedPacket.Simple.RelayPeerAddr = peerster.gossipAddr
	err = peerster.sendToKnownPeers(*receivedPacket, blacklist)
	if err != nil {
		fmt.Printf("Error: could not send packet from some other peer, reason: %s", err)
	}
	peerster.listPeers()
}

func (peerster Peerster) listen(origin Origin) {
	conn, err := peerster.createConnection(origin)
	if err != nil {
		log.Fatalf("Error: could not listen. Origin: %s, error: %s", origin, err)
	}

	for {
		buffer, originAddr, err := readFromConnection(conn)
		if err != nil {
			log.Printf("Could not read from connection, origin: %s, reason: %s", origin, err)
			break
		}
		var packet messaging.GossipPacket
		switch origin {
		case Client:
			peerster.clientReceive(buffer, packet)
		case Server:
			peerster.serverReceive(buffer, *originAddr, packet)
		}
	}
}

func (peerster *Peerster) registerNewPeer(address, peerIdentifier string, initialSeqId uint32) {
	peerster.addToKnownPeers(address)
	peerster.addToWantStruct(peerIdentifier, initialSeqId)
}

// Adds a new peer (given its unique identifier) to the peerster's want structure.
func (peerster *Peerster) addToWantStruct(peerIdentifier string, initialSeqId uint32) {
	newWant := append(peerster.want, messaging.PeerStatus{
		Identifier: peerIdentifier,
		NextID:     initialSeqId + 1,
	})
	peerster.want = newWant
}

// Called when a message is received. If the nextId of the specified peer is the same as the receivedSeqID, then nextId will be incremented
// and true will be returned - otherwise, the nextId will not be changed and false will be returned.
func (peerster *Peerster) updateWantStruct(peerIdentifier string, receivedSeqId uint32) bool {
	for i := range peerster.want {
		peer := peerster.want[i]
		if peer.Identifier != peerIdentifier {
			continue
		}
		if peer.NextID == receivedSeqId {
			peer.NextID = peer.NextID + 1
			return true
		}
		break
	}
	return false
}

// Adds the new address to the list of known peers - if it's already there, nothing happens
func (peerster *Peerster) addToKnownPeers(address string) {
	if address == peerster.gossipAddr {
		return
	}
	for i := range peerster.knownPeers {
		if address == peerster.knownPeers[i] {
			return
		}
	}
	peerster.knownPeers = append(peerster.knownPeers, address)
}

// Creates a new SimpleMessage, automatically filling out the name and relaypeeraddr fields
func (peerster Peerster) createSimpleMessage(msg string) *messaging.SimpleMessage {
	return &messaging.SimpleMessage{
		OriginalName:  peerster.name,
		RelayPeerAddr: peerster.gossipAddr,
		Contents:      msg,
	}
}

// Prints out the list of known peers in a formatted fashion
func (peerster Peerster) listPeers() {
	for i := range peerster.knownPeers {
		peer := peerster.knownPeers[i]
		fmt.Print(peer)
		if i < len(peerster.knownPeers)-1 {
			fmt.Print(",")
		} else {
			fmt.Println()
		}
	}

}

// Sends a GossipPacket to all known peers.
func (peerster Peerster) sendToKnownPeers(packet messaging.GossipPacket, blacklist []string) error {
	for i := range peerster.knownPeers {
		peer := peerster.knownPeers[i]
		if peer == peerster.gossipAddr {
			break
		}
		blacklisted := false
		for j := range blacklist {
			if peer == blacklist[j] {
				blacklisted = true
				break
			}
		}
		if blacklisted {
			break
		}
		conn, err := net.Dial("udp4", peer)
		if err != nil {
			return err
		}
		packetBytes, err := protobuf.Encode(&packet)
		if err != nil {
			return err
		}
		_, err = conn.Write(packetBytes)
		if err != nil {
			return err
		}
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
	//fmt.Println(peerster.String())
	go peerster.listen(Server)
	peerster.listen(Client)
}
