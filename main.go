package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/tormey97/Peerster/messaging"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

type Origin int

const (
	Client Origin = iota
	Server
)

//TODO big monolith split up pls

//TODO should we create a wrapper struct for other known peers?
// What this would mean is that each other peer would have certain properties, including
// (Address, identifier, rumormongeringWith)
// probably not worth
type Peerster struct {
	UIPort             string
	GossipAddress      string
	KnownPeers         []string
	Name               string
	Simple             bool
	Want               []messaging.PeerStatus
	MsgSeqNumber       int
	ReceivedMessages   map[string][]messaging.RumorMessage
	RumormongeringWith map[string][]bool
}

func (peerster Peerster) String() string {
	return fmt.Sprintf(
		"UIPort: %s, GossipAddress: %s, KnownPeers: %s, Name: %s, Simple: %t", peerster.UIPort, peerster.GossipAddress, peerster.KnownPeers, peerster.Name, peerster.Simple)
}

func (peerster Peerster) createConnection(origin Origin) (net.UDPConn, error) {
	var addr string
	switch origin {
	case Client:
		addr = "127.0.0.1:" + peerster.UIPort
	case Server:
		addr = peerster.GossipAddress
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

func (peerster Peerster) clientReceive(buffer []byte) {
	fmt.Println("CLIENT MESSAGE " + string(buffer))

	if peerster.Simple {
		packet := messaging.GossipPacket{Simple: peerster.createSimpleMessage(string(buffer))}
		err := peerster.sendToKnownPeers(packet, []string{})
		if err != nil {
			fmt.Printf("Error: could not send receivedPacket from client, reason: %s", err)
		}
	} else {
		rumor := messaging.RumorMessage{
			Origin: peerster.Name,
			ID:     2,
			Text:   string(buffer),
		}
		peerster.handleIncomingRumor(&rumor, net.UDPAddr{})
	}
}

// Handles an incoming rumor message. A zero-value originAddr means the message came from a client.
func (peerster Peerster) handleIncomingRumor(rumor *messaging.RumorMessage, originAddr net.UDPAddr) {
	if rumor == nil {
		return
	}
	peerster.addToWantStruct(rumor.Origin, rumor.ID)
	peerster.addToReceivedMessages(*rumor)
	isNew := peerster.updateWantStruct(rumor.Origin, rumor.ID)
	if isNew {
		err := peerster.sendToRandomPeer(messaging.GossipPacket{Rumor: rumor}, []string{})
		if err != nil {
			fmt.Printf("Warning: Could not send to random peer. Reason: %s", err)
		}
	}
	// TODO Status packet shite?
}

// Creates a map origin -> want
// TODO should be a method
func createWantMap(want []messaging.PeerStatus) (wantMap map[string]messaging.PeerStatus) {
	for i := range want {
		peerWant := want[i]
		wantMap[peerWant.Identifier] = peerWant
	}
	return
}

// Returns a slice of the missing messages that you have and another peer doesn't
func (peerster Peerster) getMissingMessages(theirNextId, myNextId uint32, origin string) (messages []messaging.RumorMessage) {
	for i := theirNextId; theirNextId < myNextId; i++ {
		messages = append(messages, peerster.ReceivedMessages[origin][i])
	}
	return
}

func (peerster Peerster) handleIncomingStatusPacket(packet *messaging.StatusPacket, originAddr net.UDPAddr) {
	if packet == nil {
		return
	}
	wantMap := createWantMap(peerster.Want)
	for i := range packet.Want {
		otherPeerWant := packet.Want[i]
		myWant := wantMap[otherPeerWant.Identifier]
		if myWant == (messaging.PeerStatus{}) {
			return //this situation means we don't have the peer registered, idk what to do then, add to list of peers?
		}

		statusPacket := messaging.StatusPacket{Want: peerster.Want}
		gossipPacket := messaging.GossipPacket{Status: &statusPacket}
		if myWant.NextID > otherPeerWant.NextID {
			// He's out of date, we transmit messages hes missing
			messages := peerster.getMissingMessages(otherPeerWant.NextID, myWant.NextID, otherPeerWant.Identifier)
			nextMsg := messages[0]
			// peerster.send(nextMsg, addr)
			//TODO what do i do with the messages, apparently you send "one" (in bold) message.
			// but that's weird, no? what about hte rest of the messages

			//TODO open a "session"/"connection" with another peer. You need to track the state here, because
			// one takes different actions based on whether one is the rumormonger or not
			// is it enough to just have a boolean "rumormongering-with" flag? problem is that this means you would treat
			// every statuspacket coming from a specific peer as if he is not rumormongering with you, but is that necessarily
			// the truth?
		} else if myWant.NextID == otherPeerWant.NextID {
			// We are up to date, so we ACK with a statuspacket? not sure
			/*TODO ok so apparently: sending peer (rumormonger) flips a coin, either picks a random peer to send
			the rumor message to or does nothing*/
		} else if myWant.NextID < otherPeerWant.NextID {
			// I'm out of date, we send him our status packet saying we are OOD, he should send us msgs
			peerster.sendToPeer(originAddr.String(), gossipPacket, []string{})
		}
	}
}

func (peerster Peerster) chooseRandomPeer() (string, error) {
	var validPeers []string
	for i := range peerster.KnownPeers {
		peer := peerster.KnownPeers[i]
		if peer != peerster.GossipAddress {
			validPeers = append(validPeers, peer)
		}
	}
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	if validPeers == nil {
		return "", errors.New("slice of valid peers is empty/nil")
	}
	return validPeers[r.Intn(len(validPeers))], nil
}

func (peerster Peerster) serverReceive(buffer []byte, originAddr net.UDPAddr) {
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
	receivedPacket.Simple.RelayPeerAddr = peerster.GossipAddress //TODO this line might not be necessary after part1
	if !peerster.Simple {
		peerster.handleIncomingRumor(receivedPacket.Rumor, originAddr)
		peerster.handleIncomingStatusPacket(receivedPacket.Status, originAddr)
	} else {
	}
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
		switch origin {
		case Client:
			peerster.clientReceive(buffer)
		case Server:
			peerster.serverReceive(buffer, *originAddr)
		}
	}
}

func (peerster *Peerster) registerNewPeer(address, peerIdentifier string, initialSeqId uint32) {
	peerster.addToKnownPeers(address)
	peerster.addToWantStruct(peerIdentifier, initialSeqId)
}

// Adds a new message to the list of received messages, if it has not already been received.
// Returns a boolean signifying whether the rumor was new or not
func (peerster *Peerster) addToReceivedMessages(rumor messaging.RumorMessage) bool {
	messagesFromPeer := peerster.ReceivedMessages[rumor.Origin]
	if messagesFromPeer == nil {
		peerster.ReceivedMessages[rumor.Origin] = []messaging.RumorMessage{}
		messagesFromPeer = peerster.ReceivedMessages[rumor.Origin]
	}
	message := messagesFromPeer[rumor.ID]
	if message != (messaging.RumorMessage{}) {
		// zero value, so this message is not received before
		messagesFromPeer[rumor.ID] = rumor
		return true
	}
	return false
}

// Adds a new peer (given its unique identifier) to the peerster's Want structure.
func (peerster *Peerster) addToWantStruct(peerIdentifier string, initialSeqId uint32) {
	newWant := append(peerster.Want, messaging.PeerStatus{
		Identifier: peerIdentifier,
		NextID:     initialSeqId + 1,
	})
	peerster.Want = newWant
}

// Called when a message is received. If the nextId of the specified peer is the same as the receivedSeqID, then nextId will be incremented
// and true will be returned - otherwise, the nextId will not be changed and false will be returned.
func (peerster *Peerster) updateWantStruct(peerIdentifier string, receivedSeqId uint32) bool {
	for i := range peerster.Want {
		peer := peerster.Want[i]
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
	if address == peerster.GossipAddress {
		return
	}
	for i := range peerster.KnownPeers {
		if address == peerster.KnownPeers[i] {
			return
		}
	}
	peerster.KnownPeers = append(peerster.KnownPeers, address)
}

func (peerster *Peerster) hasReceivedRumor(origin string, seqId uint32) bool {
	for i := range peerster.Want {
		peer := peerster.Want[i]
		if peer.Identifier == origin {
			if seqId < peer.NextID {
				return true
			} else {
				return false
			}
		}
	}
	return false
}

// Creates a new SimpleMessage, automatically filling out the Name and relaypeeraddr fields
func (peerster Peerster) createSimpleMessage(msg string) *messaging.SimpleMessage {
	return &messaging.SimpleMessage{
		OriginalName:  peerster.Name,
		RelayPeerAddr: peerster.GossipAddress,
		Contents:      msg,
	}
}

// Prints out the list of known peers in a formatted fashion
func (peerster Peerster) listPeers() {
	for i := range peerster.KnownPeers {
		peer := peerster.KnownPeers[i]
		fmt.Print(peer)
		if i < len(peerster.KnownPeers)-1 {
			fmt.Print(",")
		} else {
			fmt.Println()
		}
	}

}

func (peerster Peerster) sendToPeer(peer string, packet messaging.GossipPacket, blacklist []string) error {
	for j := range blacklist {
		if peer == blacklist[j] {
			return fmt.Errorf("peer %q is blacklisted")
		}
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

func (peerster Peerster) sendToRandomPeer(packet messaging.GossipPacket, blacklist []string) error {
	peer, err := peerster.chooseRandomPeer()
	if err != nil {
		fmt.Printf("Could not choose random peer, reason: %s", err)
		return err
	}
	return peerster.sendToPeer(peer, packet, blacklist)
}

// Sends a GossipPacket to all known peers.
func (peerster Peerster) sendToKnownPeers(packet messaging.GossipPacket, blacklist []string) error {
	for i := range peerster.KnownPeers {
		peer := peerster.KnownPeers[i]
		if peer == peerster.GossipAddress {
			break
		}
		err := peerster.sendToPeer(peer, packet, blacklist)
		if err != nil {
			fmt.Printf("Could not send to peer %q, reason: %s", peer, err)
		}
	}
	return nil
}

func createPeerster() Peerster {
	UIPort := flag.String("UIPort", "8080", "the port the client uses to communicate with peerster")
	gossipAddr := flag.String("GossipAddress", "127.0.0.1:5000", "the address of the peerster")
	name := flag.String("Name", "nodeA", "the Name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("Simple", true, "Simple mode")
	flag.Parse()
	return Peerster{
		UIPort:        *UIPort,
		GossipAddress: *gossipAddr,
		KnownPeers:    strings.Split(*peers, ","),
		Name:          *name,
		Simple:        *simple,
	}
}

func main() {
	peerster := createPeerster()
	//fmt.Println(peerster.String())
	go peerster.listen(Server)
	peerster.listen(Client)
}
