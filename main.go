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
	UIPort                 string
	GossipAddress          string
	KnownPeers             []string
	Name                   string
	Simple                 bool
	Want                   []messaging.PeerStatus
	MsgSeqNumber           uint32
	ReceivedMessages       map[string][]messaging.RumorMessage
	RumormongeringSessions map[string]messaging.RumormongeringSession //TODO is this necessary?
	Conn                   net.UDPConn
}

func stringAddrToUDPAddr(addr string) net.UDPAddr {
	ipAndPort := strings.Split(addr, ":")
	port, err := strconv.Atoi(ipAndPort[1])
	ip := strings.Split(ipAndPort[0], ".")
	ipByte := []byte{}
	for i := range ip {
		ipInt, err := strconv.Atoi(ip[i])
		if err != nil {
			break
		}
		ipByte = append(ipByte, byte(ipInt))
	}
	if err != nil {
		return net.UDPAddr{}
	}
	return net.UDPAddr{
		IP:   ipByte,
		Port: port,
		Zone: "",
	}
}

func (peerster *Peerster) String() string {
	return fmt.Sprintf(
		"UIPort: %s, GossipAddress: %s, KnownPeers: %s, Name: %s, Simple: %t", peerster.UIPort, peerster.GossipAddress, peerster.KnownPeers, peerster.Name, peerster.Simple)
}

func (peerster *Peerster) createConnection(origin Origin) (net.UDPConn, error) {
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
	fmt.Printf("Amount of bytes read: %s | from: %s ", n, originAddr.String())

	buffer = buffer[:n]
	return buffer, originAddr, nil
}

func (peerster *Peerster) clientReceive(buffer []byte) {
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
			ID:     peerster.MsgSeqNumber,
			Text:   string(buffer),
		}
		peerster.MsgSeqNumber = peerster.MsgSeqNumber + 1
		peerster.handleIncomingRumor(&rumor, stringAddrToUDPAddr(peerster.GossipAddress))
	}
}

func (peerster *Peerster) startRumormongeringSession(peer string, message messaging.RumorMessage) error {
	session := peerster.RumormongeringSessions[peer]
	fmt.Printf("Starting sessoin, active: %b, timeleft: %v, message: %s, peer: %s, \n", session.Active, session.TimeLeft, message, peer)
	if !session.Active {
		session.Active = true
		session.ResetTimer()
		session.Message = message
		peerster.RumormongeringSessions[peer] = session
		fmt.Printf("printing session, %s \n", session)
		go func() {
			for peerster.RumormongeringSessions[peer].TimeLeft > 0 {
				session = peerster.RumormongeringSessions[peer]
				session.DecrementTimer()
				peerster.RumormongeringSessions[peer] = session
				time.Sleep(1000 * time.Millisecond)
			}
			fmt.Printf("SESSION TIMEOUT, PEER: %s \n", peer)
			session.Active = false
		}()
	} else {
		session.ResetTimer()
		return fmt.Errorf("attempted to start a rumormongering session with %q, but one was already active", peer)
	}
	return nil
}

// Handles an incoming rumor message. A zero-value originAddr means the message came from a client.
func (peerster *Peerster) handleIncomingRumor(rumor *messaging.RumorMessage, originAddr net.UDPAddr) {
	if rumor == nil {
		return
	}
	peerster.addToWantStruct(rumor.Origin, rumor.ID)
	peerster.addToReceivedMessages(*rumor)
	isNew := peerster.updateWantStruct(rumor.Origin, rumor.ID)
	isFromMyself := originAddr.String() == peerster.GossipAddress
	if isNew || isFromMyself {
		peer, err := peerster.sendToRandomPeer(messaging.GossipPacket{Rumor: rumor}, []string{})
		if err != nil {
			fmt.Printf("Warning: Could not send to random peer. Reason: %s", err)
		}
		if isFromMyself { // We sent the message, so we say we are now rumormongering with this guy
			err := peerster.startRumormongeringSession(peer, *rumor)
			if err != nil {
				fmt.Printf("Was not able to start rumormongering session, reason: %s", err)
			}
		}
	}
	if !isFromMyself {
		err := peerster.sendStatusPacket(originAddr.String())
		fmt.Printf("Sending status packet to %s", originAddr.String())
		if err != nil {
			fmt.Printf("Could not send status packet to %s, reason: %s", originAddr.String(), err)
		}
	}
}

// Creates a map origin -> want
// TODO should be a method
func createWantMap(want []messaging.PeerStatus) (wantMap map[string]messaging.PeerStatus) {
	wantMap = map[string]messaging.PeerStatus{}
	for i := range want {
		peerWant := want[i]
		wantMap[peerWant.Identifier] = peerWant
	}
	return
}

// Returns a slice of the missing messages that you have and another peer doesn't
func (peerster *Peerster) getMissingMessages(theirNextId, myNextId uint32, origin string) (messages []messaging.RumorMessage) {
	fmt.Printf("TheirNext: %v, myNext: %v, origin: %q", theirNextId, myNextId, origin)
	for i := theirNextId - 1; i < myNextId-1; i++ {
		fmt.Println("i: ", i)
		messages = append(messages, peerster.ReceivedMessages[origin][i])
	}
	return
}

func (peerster *Peerster) sendStatusPacket(peer string) error {
	packet := messaging.GossipPacket{
		Status: &messaging.StatusPacket{Want: peerster.Want},
	}
	fmt.Printf("Sending a status packet to %s \n", peer)
	return peerster.sendToPeer(peer, packet, []string{})
}

func (peerster *Peerster) handleIncomingStatusPacket(packet *messaging.StatusPacket, originAddr net.UDPAddr) {
	if packet == nil {
		return
	}
	wantMap := createWantMap(peerster.Want)
	for i := range packet.Want {
		otherPeerWant := packet.Want[i]
		myWant := wantMap[otherPeerWant.Identifier]
		if myWant == (messaging.PeerStatus{}) {
			return //TODO this situation means we don't have the peer registered, idk what to do then, add to list of peers?
		}
		statusPacket := messaging.StatusPacket{Want: peerster.Want}
		gossipPacket := messaging.GossipPacket{Status: &statusPacket}
		synced := false
		if myWant.NextID > otherPeerWant.NextID {
			// He's out of date, we transmit messages hes missing (for this particular peer)
			messages := peerster.getMissingMessages(otherPeerWant.NextID, myWant.NextID, otherPeerWant.Identifier)
			nextMsg := messages[0]
			err := peerster.sendToPeer(originAddr.String(), messaging.GossipPacket{
				Rumor: &nextMsg,
			}, []string{})
			if err != nil {
				fmt.Printf("Could not send missing rumor to peer, reason: %s", err)
			}
			break
		} else if myWant.NextID < otherPeerWant.NextID {
			// I'm out of date, we send him our status packet saying we are OOD, he should send us msgs
			err := peerster.sendToPeer(originAddr.String(), gossipPacket, []string{})
			if err != nil {
				fmt.Printf("Could not send statuspacket. Reason: %s", err)
			}
		} else {
			synced = true
		}
		if synced {
			fmt.Printf("SYCNED. Flipping coin.")
			session := peerster.RumormongeringSessions[originAddr.String()]
			if session.Active && peerster.considerRumormongering() {
				fmt.Println("We rumormongering now. Session details: ")
				fmt.Printf("Active: %b, Originalmessage: %q, TimeLeft: %v", session.Active, session.Message, session.TimeLeft)
				peerster.handleIncomingRumor(&session.Message, stringAddrToUDPAddr(peerster.GossipAddress))
			}
		}
	}
}

func (peerster *Peerster) considerRumormongering() bool {
	num := rand.Intn(2)
	fmt.Printf("RANDOM NUMBER: %v", num)
	return num > 0
}

func (peerster *Peerster) chooseRandomPeer() (string, error) {
	var validPeers []string
	for i := range peerster.KnownPeers {
		peer := peerster.KnownPeers[i]
		if peer != peerster.GossipAddress {
			validPeers = append(validPeers, peer)
		}
	}
	if validPeers == nil {
		return "", errors.New("slice of valid peers is empty/nil")
	}
	return validPeers[rand.Intn(len(validPeers))], nil
}

func (peerster *Peerster) serverReceive(buffer []byte, originAddr net.UDPAddr) {
	receivedPacket := &messaging.GossipPacket{}
	err := protobuf.Decode(buffer, receivedPacket)
	if err != nil {
		fmt.Printf("Error: could not decode packet, reason: %s", err)
	}
	addr := originAddr.IP.String() + ":" + strconv.Itoa(originAddr.Port)
	if receivedPacket.Simple != nil && receivedPacket.Simple.RelayPeerAddr != "" {
		addr = receivedPacket.Simple.RelayPeerAddr
	}
	peerster.addToKnownPeers(addr)
	if !peerster.Simple {
		peerster.handleIncomingRumor(receivedPacket.Rumor, originAddr)
		peerster.handleIncomingStatusPacket(receivedPacket.Status, originAddr)
	} else {
		//TODO Handle SimpleMessage and Rumor cases differently. If it's a simplemessage, the relay origin addr is probably inside the message

		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s \n", receivedPacket.Simple.OriginalName, receivedPacket.Simple.RelayPeerAddr, receivedPacket.Simple.Contents)
		blacklist := []string{addr}                                  // we won't send a message to these peers
		receivedPacket.Simple.RelayPeerAddr = peerster.GossipAddress //TODO this line might not be necessary after part1
		err = peerster.sendToKnownPeers(*receivedPacket, blacklist)
		if err != nil {
			fmt.Printf("Error: could not send packet from some other peer, reason: %s", err)
		}
	}
	peerster.listPeers()
}

func (peerster *Peerster) listen(origin Origin) {
	conn, err := peerster.createConnection(origin)
	if err != nil {
		log.Fatalf("Error: could not listen. Origin: %s, error: %s", origin, err)
	}
	if origin == Server {
		peerster.Conn = conn
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
	fmt.Printf("RumorID: %v, lenmsgs: %v, origin: %s \n", rumor.ID, len(messagesFromPeer), rumor.Origin)
	if int(rumor.ID)-1 == len(messagesFromPeer) {
		fmt.Println("We get to this positoin, whats up", rumor.ID, rumor.Origin, len(messagesFromPeer))
		peerster.ReceivedMessages[rumor.Origin] = append(peerster.ReceivedMessages[rumor.Origin], rumor)
		return true
	}
	return false
}

// Adds a new peer (given its unique identifier) to the peerster's Want structure.
func (peerster *Peerster) addToWantStruct(peerIdentifier string, initialSeqId uint32) { //TODO remove initialseqid
	for i := range peerster.Want {
		if peerster.Want[i].Identifier == peerIdentifier {
			return
		}
	}
	peerster.Want = append(peerster.Want, messaging.PeerStatus{
		Identifier: peerIdentifier,
		NextID:     1,
	})
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
			peerster.Want[i].NextID += 1
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
func (peerster *Peerster) createSimpleMessage(msg string) *messaging.SimpleMessage {
	return &messaging.SimpleMessage{
		OriginalName:  peerster.Name,
		RelayPeerAddr: peerster.GossipAddress,
		Contents:      msg,
	}
}

// Prints out the list of known peers in a formatted fashion
func (peerster *Peerster) listPeers() {
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
	peerAddr := stringAddrToUDPAddr(peer)
	packetBytes, err := protobuf.Encode(&packet)
	if err != nil {
		return err
	}
	n, err := peerster.Conn.WriteToUDP(packetBytes, &peerAddr)
	fmt.Printf("Amount of bytes written: %s | written to: %s", n, peerAddr.String())
	if err != nil {
		return err
	}
	return nil
}

func (peerster *Peerster) sendToRandomPeer(packet messaging.GossipPacket, blacklist []string) (string, error) {
	peer, err := peerster.chooseRandomPeer()
	if err != nil {
		fmt.Printf("Could not choose random peer, reason: %s", err)
		return "", err
	}
	return peer, peerster.sendToPeer(peer, packet, blacklist)
}

// Sends a GossipPacket to all known peers.
func (peerster *Peerster) sendToKnownPeers(packet messaging.GossipPacket, blacklist []string) error {
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
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "the address of the peerster")
	name := flag.String("name", "nodeA", "the Name of the node")
	peers := flag.String("peers", "", "known peers")
	simple := flag.Bool("simple", false, "Simple mode")
	flag.Parse()
	return Peerster{
		UIPort:                 *UIPort,
		GossipAddress:          *gossipAddr,
		KnownPeers:             strings.Split(*peers, ","),
		Name:                   *name,
		Simple:                 *simple,
		RumormongeringSessions: map[string]messaging.RumormongeringSession{},
		ReceivedMessages:       map[string][]messaging.RumorMessage{},
		MsgSeqNumber:           1,
		Want:                   []messaging.PeerStatus{},
		Conn:                   net.UDPConn{},
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	peerster := createPeerster()
	fmt.Println(peerster.String())
	addr := stringAddrToUDPAddr(peerster.GossipAddress)
	fmt.Println(addr.String(), string(addr.IP), peerster.GossipAddress)
	go peerster.listen(Server)
	peerster.listen(Client)
}
