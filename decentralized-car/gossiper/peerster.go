package gossiper

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/dedis/protobuf"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
)

type Origin int

const (
	Client Origin = iota
	Server Origin = iota
)

type Peerster struct {
	UIPort           string
	GossipAddress    string
	KnownPeers       []string
	Name             string
	Simple           bool
	AntiEntropyTimer int
	Want             []messaging.PeerStatus
	MsgSeqNumber     uint32
	ReceivedMessages struct { //TODO is there a nice way to make a generic mutex map type, instead of having to do this every time?
		Map   map[string][]messaging.RumorMessage
		Mutex sync.RWMutex
	}
	ReceivedPrivateMessages struct {
		Map   map[string][]messaging.PrivateMessage
		Mutex sync.RWMutex
	}
	RumormongeringSessions messaging.AtomicRumormongeringSessionMap
	Conn                   net.UDPConn
	RTimer                 int
	NextHopTable           struct {
		Map   map[string]string
		Mutex sync.RWMutex
	}
	SharedFiles struct {
		Map   map[string]SharedFile
		Mutex sync.RWMutex
	}
	FileChunks struct {
		Map   map[string][]byte
		Mutex sync.RWMutex
	}
	RecentSearchRequests struct {
		Array []messaging.SearchRequest
		Mutex sync.RWMutex
	}
	FileSearchSessions
	DownloadingFiles
	FileMatches
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

func (peerster *Peerster) clientReceive(message messaging.Message) {
	destinationString := ""
	if message.Destination != nil && *message.Destination != "" {
		destinationString = " dest " + *message.Destination
	}
	fmt.Println("CLIENT MESSAGE " + message.Text + destinationString)
	if peerster.Simple {
		packet := messaging.GossipPacket{Simple: peerster.createSimpleMessage(message.Text)}
		err := peerster.sendToKnownPeers(packet, []string{})
		if err != nil {
			fmt.Printf("Error: could not send receivedPacket from client, reason: %s \n", err)
		}
	} else {
		if message.File != "" && message.Request == "" {
			peerster.shareFile(message.File)
		} else if message.Request != "" && message.File != "" && message.Destination != nil && *message.Destination != "" {
			// We decode the hexadecimal metafile request
			dst := make([]byte, hex.DecodedLen(len([]byte(message.Request))))
			_, err := hex.Decode(dst, []byte(message.Request))
			if err != nil {
				fmt.Printf("Warning: Invalid input %s from client when requesting file \n", message.Request)
				return
			}
			file := FileBeingDownloaded{
				MetafileHash:   dst,
				Channel:        make(chan messaging.DataReply),
				CurrentChunk:   0,
				Metafile:       nil,
				DownloadedData: nil,
				FileName:       message.File,
			}
			peerster.downloadData([]string{*message.Destination}, file)
		} else if (message.Destination == nil || *message.Destination == "") && message.File != "" && message.Request != "" {
			peerster.FileMatches.Mutex.RLock()
			decodedRequest, _ := hex.DecodeString(message.Request)
			_, ok := peerster.FileMatches.Map[string(decodedRequest)]
			for i := range peerster.FileMatches.Map {
				utils.DebugPrintln([]byte(i), []byte(message.Request), "????????", message.Request)
			}
			peerster.FileMatches.Mutex.RUnlock()

			if !ok {
				utils.DebugPrintln(" we don't have a search done for that file, no match.")
				return // TODO we don't have a search done for that file, no match.
			}
			if !peerster.FileMatches.isFullyMatched(decodedRequest) {
				utils.DebugPrintln("We arent done with the search yet/file isnt fully matched")
				return // TODO We arent done with the search yet/file isnt fully matched
			}

			// If we pass the checks, we need to perform a file download, but with a dynamic destination.
			// First step: We create an order in which we will downlaod the chunks..
			order, err := peerster.FileMatches.createDownloadChain(decodedRequest)
			utils.DebugPrintln("WE JUST DID", order)
			if err != nil {
				utils.DebugPrintln(err)
				return
			}
			dst := make([]byte, hex.DecodedLen(len([]byte(message.Request))))
			_, _ = hex.Decode(dst, []byte(message.Request))
			file := FileBeingDownloaded{
				MetafileHash:   dst,
				Channel:        make(chan messaging.DataReply),
				CurrentChunk:   0,
				Metafile:       nil,
				DownloadedData: nil,
				FileName:       message.File,
			}
			peerster.downloadData(order, file)

		} else if message.Keywords != nil {
			utils.DebugPrintln("SEARCHING")
			peerster.searchForFiles(message.Keywords, message.Budget)
		} else if message.Destination == nil || *message.Destination == "" {
			peerster.sendNewRumorMessage(message.Text)
		} else {
			peerster.sendNewPrivateMessage(message)
		}
	}
}

func (peerster *Peerster) sendNewRumorMessage(text string) {
	rumor := messaging.RumorMessage{
		Origin: peerster.Name,
		ID:     peerster.MsgSeqNumber,
		Text:   text,
	}
	peerster.MsgSeqNumber = peerster.MsgSeqNumber + 1
	peerster.handleIncomingRumor(&rumor, messaging.StringAddrToUDPAddr(peerster.GossipAddress), false)
}

func (peerster *Peerster) sendNewPrivateMessage(msg messaging.Message) {
	private := messaging.PrivateMessage{
		Origin:      peerster.Name,
		ID:          0,
		Text:        msg.Text,
		Destination: *msg.Destination,
		HopLimit:    10,
	}
	peerster.handleIncomingPrivateMessage(&private, messaging.StringAddrToUDPAddr(peerster.GossipAddress))
}
func (peerster *Peerster) startRumormongeringSession(peer string, message messaging.RumorMessage) error {
	session, ok := peerster.RumormongeringSessions.GetSession(peer)
	if !ok || session.Active == false {
		session = messaging.RumormongeringSession{
			Message: message,
			Channel: make(chan bool),
			Active:  false,
			Mutex:   sync.RWMutex{},
		}
		peerster.RumormongeringSessions.SetSession(peer, session)
	}

	//fmt.Printf("Starting sessoin, active: %b, timeleft: %v, message: %s, peer: %s, \n", session.Active, session.Message, message, peer)
	if peerster.RumormongeringSessions.ActivateSession(peer) {
		go func() {
			session, ok := peerster.RumormongeringSessions.GetSession(peer)
			if !ok {
				return
			}
			timedOut := false
			interrupted := false
			for !timedOut && !interrupted {
				select {
				case interrupt := <-session.Channel:
					if interrupt {
						interrupted = true
					}
				case <-time.After(10 * time.Second):
					timedOut = true
				}
			}
			if timedOut {
				session, ok := peerster.RumormongeringSessions.GetSession(peer)
				peerster.RumormongeringSessions.DeactivateSession(peer)
				if ok {
					peerster.handleIncomingRumor(&session.Message, messaging.StringAddrToUDPAddr(peerster.GossipAddress), false) // we rerun
				}
			}
		}()
	} else {
		peerster.RumormongeringSessions.ResetTimer(peer)
		return fmt.Errorf("attempted to start a rumormongering session with %q, but one was already active \n", peer)
	}
	return nil
}

func (peerster *Peerster) stopRumormongeringSession(peer string) {
	peerster.RumormongeringSessions.InterruptSession(peer)
	peerster.RumormongeringSessions.DeactivateSession(peer)
}

// Handles an incoming rumor message. A zero-value originAddr means the message came from a client.
// This also handles messages that are sent by the peerster itself
func (peerster *Peerster) handleIncomingRumor(rumor *messaging.RumorMessage, originAddr net.UDPAddr, coinflip bool) string {
	if rumor == nil {
		return ""
	}
	if *rumor == (messaging.RumorMessage{}) {
		return ""
	}
	//if rumor.Text != "" {
	//fmt.Printf("RUMOR origin %s from %s ID %v contents %s \n", rumor.Origin, originAddr.String(), rumor.ID, rumor.Text)
	//}
	peerster.addToWantStruct(rumor.Origin, rumor.ID)
	peerster.addToReceivedMessages(*rumor)
	if rumor.Text != "" {
		fmt.Printf("DSDV %s %s \n", rumor.Origin, originAddr.String())
	}
	isNew := peerster.updateWantStruct(rumor.Origin, rumor.ID)
	if isNew {
		// We add the originaddress to the next hop table if it's a new message
		peerster.addToNextHopTable(*rumor, originAddr.String())
	}
	isFromMyself := originAddr.String() == peerster.GossipAddress
	peer := ""
	//fmt.Printf("ISNEW: %v ISFROMMYSELF: %v COINFLIP: %v", isNew, isFromMyself, coinflip)
	if isNew || isFromMyself || coinflip {
		selectedPeer, err := peerster.sendToRandomPeer(messaging.GossipPacket{Rumor: rumor}, []string{})
		peer = selectedPeer
		if err != nil {
			fmt.Printf("Warning: Could not send to random peer. Reason: %s \n", err)
		}
		peerster.stopRumormongeringSession(peer)
		err = peerster.startRumormongeringSession(peer, *rumor)
		if err != nil {
			fmt.Printf("Was not able to start rumormongering session. Coinflip: %v, reason: %s \n", coinflip, err)
		}
	}
	if !isFromMyself {
		err := peerster.sendStatusPacket(originAddr.String())
		//fmt.Printf("Sending status packet to %s \n", originAddr.String())
		if err != nil {
			fmt.Printf("Could not send status packet to %s, reason: %s \n", originAddr.String(), err)
		}
	}
	return peer
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
	//fmt.Printf("TheirNext: %v, myNext: %v, origin: %q", theirNextId, myNextId, origin)
	peerster.ReceivedMessages.Mutex.RLock()
	for i := theirNextId - 1; i < myNextId-1; i++ {
		//fmt.Println("i: ", i)
		messages = append(messages, peerster.ReceivedMessages.Map[origin][i])
	}
	peerster.ReceivedMessages.Mutex.RUnlock()
	return
}

func (peerster *Peerster) sendStatusPacket(peer string) error {
	packet := messaging.GossipPacket{
		Status: &messaging.StatusPacket{Want: peerster.Want},
	}
	return peerster.sendToPeer(peer, packet, []string{})
}

func (peerster *Peerster) handleIncomingPrivateMessage(message *messaging.PrivateMessage, originAddr net.UDPAddr) {
	if message == nil {
		return
	}
	if message.Destination == peerster.Name {
		fmt.Printf("PRIVATE origin %s hop-limit %v contents %s \n", message.Origin, message.HopLimit, message.Text)
		peerster.addToPrivateMessages(*message)
	} else {
		message.HopLimit--
		if message.HopLimit == 0 {
			return
		}
		peerster.nextHopRoute(&messaging.GossipPacket{Private: message}, message.Destination)
	}
}

func (peerster *Peerster) handleIncomingStatusPacket(packet *messaging.StatusPacket, originAddr net.UDPAddr) {
	if packet == nil {
		return
	}
	// Printing for the automated tests
	statusString := fmt.Sprintf("STATUS from %s ", originAddr.String())
	for i := range packet.Want {
		statusString += fmt.Sprintf("peer %s nextID %v ", packet.Want[i].Identifier, packet.Want[i].NextID)
	}
	//fmt.Println(statusString)
	// End printing
	//peerster.RumormongeringSessions.ResetTimer(originAddr.String()) //TODO verify that this makes sense
	wantMap := createWantMap(peerster.Want)

	//Handles the case where the other peer doesn't even know about a certain peer we know about
	for i := range peerster.Want {
		identifier := peerster.Want[i].Identifier
		found := false
		for j := range packet.Want {
			if packet.Want[j].Identifier == identifier {
				found = true
				break
			}
		}

		if !found {
			// Other peer doesn't even know about one of the peers in our want, so we just send him this peer's first message here
			//fmt.Printf("Other peer doesn't know about a peer. Sending first message. \n")
			peerster.ReceivedMessages.Mutex.RLock()
			if len(peerster.ReceivedMessages.Map[identifier]) >= 1 {
				firstMessage := peerster.ReceivedMessages.Map[identifier][0]
				peerster.ReceivedMessages.Mutex.RUnlock()
				err := peerster.sendToPeer(originAddr.String(), messaging.GossipPacket{
					Rumor: &firstMessage,
				}, []string{})
				if err != nil {
					fmt.Printf("Could not send first message to another peer, reason: %s \n", err)
				}
				return
			}
			peerster.ReceivedMessages.Mutex.RUnlock()
		}
	}
	statusPacket := messaging.StatusPacket{Want: peerster.Want}
	gossipPacket := messaging.GossipPacket{Status: &statusPacket}
	shouldSendStatusPacket := false
	for i := range packet.Want {
		otherPeerWant := packet.Want[i]
		myWant := wantMap[otherPeerWant.Identifier]
		if myWant == (messaging.PeerStatus{}) && otherPeerWant.Identifier != peerster.Name {
			//fmt.Printf("unknown peer %q encountered, should send statuspacket \n", otherPeerWant.Identifier)
			peerster.addToWantStruct(otherPeerWant.Identifier, 1)
		}
		//peerster.printMessages()
		if myWant.NextID > otherPeerWant.NextID {
			// He's out of date, we transmit messages hes missing (for this particular peer)
			messages := peerster.getMissingMessages(otherPeerWant.NextID, myWant.NextID, otherPeerWant.Identifier)
			nextMsg := messages[0]
			err := peerster.sendToPeer(originAddr.String(), messaging.GossipPacket{
				Rumor: &nextMsg,
			}, []string{})
			if err != nil {
				fmt.Printf("Could not send missing rumor to peer, reason: %s \n", err)
			}
			return
		} else if myWant.NextID < otherPeerWant.NextID {
			// I'm out of date, we send him our status packet saying we are OOD, he should send us msgs
			shouldSendStatusPacket = true
		}
	}
	if shouldSendStatusPacket {
		err := peerster.sendToPeer(originAddr.String(), gossipPacket, []string{})
		if err != nil {
			fmt.Printf("Could not send statuspacket. Reason: %s \n", err)
		}
		return
	}

	//fmt.Printf("IN SYNC WITH %s \n", originAddr.String())

	session, ok := peerster.RumormongeringSessions.GetSession(originAddr.String())
	if ok && session.Active && peerster.considerRumormongering() {
		_ = peerster.handleIncomingRumor(&session.Message, messaging.StringAddrToUDPAddr(peerster.GossipAddress), true)
		//fmt.Printf("FLIPPED COIN sending rumor to %s \n", targetAddr)
	}
	peerster.stopRumormongeringSession(originAddr.String())
}

func (peerster *Peerster) considerRumormongering() bool {
	num := rand.Intn(40)
	return num > 9
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
	num := rand.Intn(len(validPeers))
	return validPeers[num], nil
}

// Handles incoming messages from other peers.
func (peerster *Peerster) serverReceive(buffer []byte, originAddr net.UDPAddr) {
	receivedPacket := &messaging.GossipPacket{}
	err := protobuf.Decode(buffer, receivedPacket)
	if err != nil {
		fmt.Printf("Error: could not decode packet, reason: %s \n", err)
	}
	addr := originAddr.IP.String() + ":" + strconv.Itoa(originAddr.Port)
	if receivedPacket.Simple != nil && receivedPacket.Simple.RelayPeerAddr != "" {
		addr = receivedPacket.Simple.RelayPeerAddr
	}
	peerster.addToKnownPeers(addr)
	if !peerster.Simple {
		peerster.handleIncomingRumor(receivedPacket.Rumor, originAddr, false)
		peerster.handleIncomingStatusPacket(receivedPacket.Status, originAddr)
		peerster.handleIncomingPrivateMessage(receivedPacket.Private, originAddr)
		peerster.handleIncomingDataReply(receivedPacket.DataReply, originAddr)
		peerster.handleIncomingDataRequest(receivedPacket.DataRequest, originAddr)
		peerster.handleIncomingSearchRequest(receivedPacket.SearchRequest, originAddr)
		peerster.handleIncomingSearchReply(receivedPacket.SearchReply, originAddr)
	} else {
		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s \n", receivedPacket.Simple.OriginalName, receivedPacket.Simple.RelayPeerAddr, receivedPacket.Simple.Contents)
		blacklist := []string{addr} // we won't send a message to these peers
		receivedPacket.Simple.RelayPeerAddr = peerster.GossipAddress
		err = peerster.sendToKnownPeers(*receivedPacket, blacklist)
		if err != nil {
			fmt.Printf("Error: could not send packet from some other peer, reason: %s \n", err)
		}
	}
	//peerster.listPeers()
}

// For testing, prints the want structure and all received messages just to see if it$s correct
func (peerster Peerster) printMessages() {
	fmt.Println("PRINTING WANT")
	for i := range peerster.Want {
		fmt.Println(peerster.Want[i])
	}
	fmt.Println("PRINTING RECEIVED MSGS")
	peerster.ReceivedMessages.Mutex.RLock()
	defer peerster.ReceivedMessages.Mutex.RUnlock()
	for i := range peerster.ReceivedMessages.Map {
		peer := peerster.ReceivedMessages.Map[i]
		fmt.Println()
		for j := range peer {
			fmt.Println(peer[j])
		}
	}
}

// Listens to the network sockets and sends any incoming messages to be processed by the peerster.
func (peerster *Peerster) Listen(origin Origin) {
	conn, err := peerster.createConnection(origin)
	if err != nil {
		log.Fatalf("Error: could not listen. Origin: %s, error: %s \n", origin, err)
	}
	if origin == Server {
		peerster.Conn = conn
	}
	for {
		buffer, originAddr, err := messaging.ReadFromConnection(conn)
		if err != nil {
			log.Printf("Could not read from connection, origin: %s, reason: %s \n", origin, err)
			break
		}
		switch origin {
		case Client:
			msg := messaging.Message{}
			err := protobuf.Decode(buffer, &msg)
			if err != nil {
				fmt.Printf("Failed to decode message from client, reason: %s \n", err)
			} else {
				//fmt.Println("THE MESSAGE IS ", msg.Text)
				go peerster.clientReceive(msg)
			}
		case Server:
			// We start a goroutine for the processing step so we can keep reading from the socket
			go peerster.serverReceive(buffer, *originAddr)
		}
	}
}

// Adds a new peer to the list of known peers, and adds it to the peerster's Want structure
func (peerster *Peerster) registerNewPeer(address, peerIdentifier string, initialSeqId uint32) {
	peerster.addToKnownPeers(address)
	peerster.addToWantStruct(peerIdentifier, initialSeqId)
}

// Adds a private message to the list of received private messages
func (peerster *Peerster) addToPrivateMessages(private messaging.PrivateMessage) {
	peerster.ReceivedPrivateMessages.Mutex.Lock()
	defer peerster.ReceivedPrivateMessages.Mutex.Unlock()
	privateMessagesFromPeer, ok := peerster.ReceivedPrivateMessages.Map[private.Origin]
	if !ok {
		peerster.ReceivedPrivateMessages.Map[private.Origin] = []messaging.PrivateMessage{}
		privateMessagesFromPeer = peerster.ReceivedPrivateMessages.Map[private.Origin]
	}
	peerster.ReceivedPrivateMessages.Map[private.Origin] = append(privateMessagesFromPeer, private)
}

// Adds a new message to the list of received messages, if it has not already been received.
// Returns a boolean signifying whether the rumor was new or not
func (peerster *Peerster) addToReceivedMessages(rumor messaging.RumorMessage) bool {
	peerster.ReceivedMessages.Mutex.Lock()
	defer peerster.ReceivedMessages.Mutex.Unlock()
	messagesFromPeer := peerster.ReceivedMessages.Map[rumor.Origin]
	if messagesFromPeer == nil {
		peerster.ReceivedMessages.Map[rumor.Origin] = []messaging.RumorMessage{}
		messagesFromPeer = peerster.ReceivedMessages.Map[rumor.Origin]
	}
	if int(rumor.ID)-1 == len(messagesFromPeer) {
		//fmt.Println("We get to this position, whats up", rumor.ID, rumor.Origin, len(messagesFromPeer))
		peerster.ReceivedMessages.Map[rumor.Origin] = append(peerster.ReceivedMessages.Map[rumor.Origin], rumor)
		return true
	}
	return false
}

// Adds a new peer (given its unique identifier) to the peerster's Want structure.
func (peerster *Peerster) addToWantStruct(peerIdentifier string, initialSeqId uint32) { //TODO remove initialseqid
	if peerIdentifier == "" {
		return
	}
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

// Checks if a rumor has been received before using the origin and the sequence ID
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

// The method that handles sending messages over the network.
func (peerster Peerster) sendToPeer(peer string, packet messaging.GossipPacket, blacklist []string) error {
	for j := range blacklist {
		if peer == blacklist[j] {
			return fmt.Errorf("peer %q is blacklisted")
		}
	}

	if packet.Rumor != nil {
		//fmt.Printf("MONGERING with %s \n", peer)
	}
	peerAddr := messaging.StringAddrToUDPAddr(peer)
	packetBytes, err := protobuf.Encode(&packet)
	if err != nil {
		return err
	}
	_, err = peerster.Conn.WriteToUDP(packetBytes, &peerAddr)
	//fmt.Printf("Amount of bytes written: %v | written to: %s \n", n, peerAddr.String())
	if err != nil {
		return err
	}
	return nil
}

// Sends a GossipPacket to a random peer, and returns the peer the message was sent to.
func (peerster *Peerster) sendToRandomPeer(packet messaging.GossipPacket, blacklist []string) (string, error) {
	peer, err := peerster.chooseRandomPeer()
	if err != nil {
		fmt.Printf("Could not choose random peer, reason: %s \n", err)
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
			fmt.Printf("Could not send to peer %q, reason: %s \n", peer, err)
		}
	}
	return nil
}

// Starts the peerster anti-entropy process using an infinite while loop
func (peerster *Peerster) AntiEntropy() {
	go func() {
		for {
			time.Sleep(time.Duration(peerster.AntiEntropyTimer) * time.Second)
			packet := messaging.GossipPacket{Status: &messaging.StatusPacket{Want: peerster.Want}}
			_, err := peerster.sendToRandomPeer(packet, []string{})
			if err != nil {
				fmt.Printf("Antientropy failed, reason: %s \n", err)
			}
		}
	}()
}
