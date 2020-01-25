package gossiper

import (
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
	Client        Origin        = iota
	Server        Origin        = iota
	TIMEOUTCARS   time.Duration = 15
	IMAGEACCIDENT string        = "accident.jpg"
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
	CarMap           *utils.SimulatedMap
	CarPosition      utils.Position // unused?
	EndCarP          utils.Position // unused?
	PathCar          []utils.Position
	PosCarsInArea    utils.CarInfomartionList
	Newsgroups       []string
	BroadcastTimer   int
	ColisionInfo     utils.ColisionInformation
	AreaChangeSession
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
	DownloadingFiles
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

}

func (peerster *Peerster) sendNewRumorMessage(message messaging.RumorMessage) {
	message.Origin = peerster.Name
	message.ID = peerster.MsgSeqNumber
	peerster.MsgSeqNumber += 1
	peerster.handleIncomingRumor(&message, utils.StringAddrToUDPAddr(peerster.GossipAddress), false)
}

func (peerster *Peerster) sendNewPrivateMessage(message messaging.PrivateMessage) {
	message.Origin = peerster.Name
	message.ID = 0
	message.HopLimit = 20
	peerster.handleIncomingPrivateMessage(&message, utils.StringAddrToUDPAddr(peerster.GossipAddress))
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
					peerster.handleIncomingRumor(&session.Message, utils.StringAddrToUDPAddr(peerster.GossipAddress), false) // we rerun
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
	// If the message is in an appropriate newsgroup, we should handle the various subtypes of rumormessage
	if peerster.filterMessageByNewsgroup(*rumor) {
		peerster.handleIncomingAccident(*rumor)
		peerster.handleIncomingAreaChange(*rumor, originAddr.String())
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
func (peerster *Peerster) handleIncomingArea(areaMessage *messaging.AreaMessage, IPofCar string) {
	if areaMessage == nil {
		return
	}
	if *areaMessage == (messaging.AreaMessage{}) {
		return
	}
	carExists := false
	peerster.PosCarsInArea.Mutex.Lock()
	for _, carInfo := range peerster.PosCarsInArea.Slice {
		if carInfo.Origin == areaMessage.Origin {
			carInfo.Position = areaMessage.Position
			carInfo.Channel <- false
			carExists = true
		}
	}
	peerster.PosCarsInArea.Mutex.Unlock()
	if carExists == false {
		origin := areaMessage.Origin
		position := areaMessage.Position
		peerster.saveCarInAreaStructure(origin, position, IPofCar)
	}

}
func (peerster *Peerster) saveCarInAreaStructure(origin string, position utils.Position, IPofCar string) {
	peerster.PosCarsInArea.Mutex.RLock()
	for _, car := range peerster.PosCarsInArea.Slice {
		if car.Origin == origin {
			return // car already exists
		}
	}
	peerster.PosCarsInArea.Mutex.RUnlock()
	infoOfCar := &utils.CarInformation{
		Origin:   origin,
		Position: position,
		Channel:  make(chan bool),
		IPCar:    IPofCar,
	}
	peerster.PosCarsInArea.Mutex.Lock()
	peerster.PosCarsInArea.Slice = append(peerster.PosCarsInArea.Slice, infoOfCar)
	peerster.PosCarsInArea.Mutex.Unlock()
	go peerster.timeoutAreaMessage(infoOfCar)
}
func (peerster *Peerster) timeoutAreaMessage(infoOfCar *utils.CarInformation) {
	for {
		select {
		// Delete car
		case deleteCar := <-infoOfCar.Channel:
			if deleteCar {
				peerster.PosCarsInArea.Mutex.Lock()
				for index, carInfo := range peerster.PosCarsInArea.Slice {
					if carInfo.Origin == infoOfCar.Origin {
						peerster.PosCarsInArea.Slice = append(peerster.PosCarsInArea.Slice[:index], peerster.PosCarsInArea.Slice[index+1:]...)
					}
				}
				peerster.PosCarsInArea.Mutex.Unlock()
				break
			}
		case <-time.After(TIMEOUTCARS * time.Second):
			infoOfCar.Channel <- true
		}
	}
	// First message from that car
}
func (peerster *Peerster) handleIncomingResolutionM(colisionMessage *messaging.ColisionResolution, addr string) {
	if colisionMessage == nil {
		return
	}
	if *colisionMessage == (messaging.ColisionResolution{}) {
		return
	}
	//This means that this message is the response to our coin flip
	if peerster.ColisionInfo.CoinFlip != 0 {
		hisCoinFlip := colisionMessage.CoinResult
		peerster.colisionLogicManager(hisCoinFlip)

		//If we are here is because someone is colliding with us and send us his coin flip
	} else {
		// We answer him back with the coin flip
		if peerster.AreaChangeSession.Active {
			peerster.AreaChangeSession.Channel <- true // we interrupt the area change session
		}
		min := 1
		max := 7000
		coinFlip := rand.Intn(max-min+1) + min
		peerster.ColisionInfo.IPCar = addr
		peerster.ColisionInfo.CoinFlip = coinFlip
		peerster.SendNegotiationMessage()
	}

}
func (peerster *Peerster) handleIncomingAccidentMessage(alertToPolice *messaging.AlertPolice) {
	if alertToPolice == nil {
		return
	}
	// There has been an accident, so download file and go there
	file := FileBeingDownloaded{
		MetafileHash:   alertToPolice.Evidence,
		Channel:        make(chan messaging.DataReply),
		CurrentChunk:   0,
		Metafile:       nil,
		DownloadedData: nil,
		FileName:       "accident.jpg",
	}
	peerster.downloadData([]string{alertToPolice.Origin}, file)

	//Change path
	// He spawns at the middle
	startPos := utils.Position{
		X: 4,
		Y: 4,
	}
	var obstructions []utils.Position
	peerster.PathCar = CreatePath(peerster.CarMap, startPos, alertToPolice.Position, obstructions)
	// Now the police car should start movingg
}

func (peerster *Peerster) handleIncomingServerAccidentMessage(alertMessage *utils.ServerMessage) {
	if alertMessage == nil {
		return
	}
	if *alertMessage == (utils.ServerMessage{}) {
		return
	}
	if alertMessage.Type != utils.Police {
		return
	}
	// Notify the police via private message with hops and share the file
	// Notify the rest of people with rumor monger
	alert := messaging.AlertPolice{
		Position: peerster.PathCar[0],
	}
	// File indexing
	alert.Evidence = peerster.shareFile(IMAGEACCIDENT)
	alert.Origin = peerster.Name
	privateAlert := messaging.PrivateMessage{
		AlertPolice: &alert,
	}
	peerster.sendNewPrivateMessage(privateAlert)
}
func (peerster *Peerster) handleIncomingServerSpotMessage(spotMessage *utils.ServerMessage) {
	if spotMessage == nil {
		return
	}
	if *spotMessage == (utils.ServerMessage{}) {
		return
	}
	if spotMessage.Type != utils.Parking {
		return
	}
	//TODO: Publish the spot in the newsgroup and initiate a session if someone wants to ask for the spot
}
func (peerster *Peerster) colisionLogicManager(hisCoinFlip int) {
	//If our coinflip is superior we don´t have to recalculate path
	if peerster.ColisionInfo.CoinFlip > hisCoinFlip {
		//TODO call move here?
		return
		//This means that we have to move
	} else {
		var obstructions []utils.Position
		// We tell the pathfinding alogirthm that we can't go there
		obstructions = append(obstructions, peerster.PathCar[1])
		peerster.PathCar = CreatePath(peerster.CarMap, peerster.PathCar[0], peerster.PathCar[len(peerster.PathCar)-1], obstructions)
		//Reset colision object
		peerster.ColisionInfo.CoinFlip = 0
		peerster.ColisionInfo.NumberColisions = 0
		peerster.ColisionInfo.IPCar = ""
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
		peerster.handleIncomingAreaChangeResponse(*message.AreaChangeResponse)
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
		_ = peerster.handleIncomingRumor(&session.Message, utils.StringAddrToUDPAddr(peerster.GossipAddress), true)
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

	//Function only checked by police car
	if peerster.Name == utils.Police {
		peerster.handleIncomingAccidentMessage(receivedPacket.Private.AlertPolice)
	}
	//Function where the server tells the car if he has entered an accidented zone
	peerster.handleIncomingServerAccidentMessage(receivedPacket.ServerAlert)

	//Function where the server tells the car if he has entered a zone with free spot
	peerster.handleIncomingServerSpotMessage(receivedPacket.ServerAlert)

	//Function that handles a colision negotiation message
	peerster.handleIncomingResolutionM(receivedPacket.Colision, addr)
	peerster.handleIncomingRumor(receivedPacket.Rumor, originAddr, false)

	//Function that handles position of other cars in area
	peerster.handleIncomingArea(receivedPacket.Area, addr)
	peerster.handleIncomingStatusPacket(receivedPacket.Status, originAddr)
	peerster.handleIncomingPrivateMessage(receivedPacket.Private, originAddr)
	peerster.handleIncomingDataReply(receivedPacket.DataReply, originAddr)
	peerster.handleIncomingDataRequest(receivedPacket.DataRequest, originAddr)
	// peerster.handleIncomingSearchRequest(receivedPacket.SearchRequest, originAddr)
	// peerster.handleIncomingSearchReply(receivedPacket.SearchReply, originAddr)
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
		buffer, originAddr, err := utils.ReadFromConnection(conn)
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
	peerAddr := utils.StringAddrToUDPAddr(peer)
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
			continue
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
