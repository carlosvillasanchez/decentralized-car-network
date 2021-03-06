/***
* Simulation of a Decentralized Network of Autonomous Cars
* Authors:
* 	- Torstein Meyer
* 	- Fernando Monje
* 	- Carlos Villa
***/
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

	randCrytpo "crypto/rand"
	"crypto/rsa"
	keyParser "crypto/x509"

	"github.com/dedis/protobuf"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
)

type Origin int

const (
	Client           Origin        = iota
	Server           Origin        = iota
	TIMEOUTCARS      time.Duration = 15
	TIMEOUTSPOTS     time.Duration = 6
	IMAGEACCIDENT    string        = "accident.jpg"
	BroadcastTimer   int           = 1 //Each 1 second the car broadcast position
	MovementTimer    int           = 3
	AreaChangeTimer  time.Duration = 5
	ParkingNewsGroup string        = "parking"
)

type Peerster struct {
	UIPort            string
	GossipAddress     string
	KnownPeers        []string
	Name              string
	Simple            bool
	AntiEntropyTimer  int
	Want              []messaging.PeerStatus
	MsgSeqNumber      uint32
	CarMap            *utils.SimulatedMap
	CarPosition       utils.Position // unused?
	EndCarP           utils.Position // unused?
	PathCar           []utils.Position
	PosCarsInArea     utils.CarInfomartionList
	PostulatedAlready bool
	Winner            bool
	Newsgroups        []string
	BroadcastTimer    int
	Sk                rsa.PrivateKey
	Pk                rsa.PublicKey
	PolicePk          rsa.PublicKey
	WT                bool
	Verbose           bool
	TrustedCars       []string
	PksOfTrustedCars  map[string]rsa.PublicKey
	Signatures        map[string][]byte
	ColisionInfo      utils.ColisionInformation
	AreaChangeSession
	CarsInterestedSpot messaging.SpotInformation
	ReceivedMessages   struct { //TODO is there a nice way to make a generic mutex map type, instead of having to do this every time?
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
			Channel: make(chan bool, 1),
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
	if peerster.FilterMessageByNewsgroup(*rumor) {
		peerster.handleIncomingAccident(*rumor)
		peerster.handleIncomingAreaChange(*rumor)
		//Function where you recieve a spot publication and can request it
		peerster.handleIncomingFreeSpotMessage(*rumor)
	}
	isFromMyself := originAddr.String() == peerster.GossipAddress
	peer := ""
	//fmt.Printf("ISNEW: %v ISFROMMYSELF: %v COINFLIP: %v", isNew, isFromMyself, coinflip)
	if isNew || isFromMyself || coinflip {
		selectedPeer, err := peerster.sendToRandomPeer(messaging.GossipPacket{Rumor: rumor}, []string{})
		peer = selectedPeer
		if err != nil {
			//fmt.Printf("Warning: Could not send to random peer. Reason: %s \n", err)
			//fmt.Println("peer: ", peer)
			//fmt.Printf("%+v\n", messaging.GossipPacket{Rumor: rumor})
			return ""
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
	if areaMessage == nil || areaMessage.Origin == peerster.Name {
		return
	}
	if *areaMessage == (messaging.AreaMessage{}) {
		return
	}
	//If the other car is in your area, or the next point where you want to go (another area you want to enter)
	if len(peerster.PathCar) > 1 {
		if (utils.AreaPositioner(areaMessage.Position) == utils.AreaPositioner(peerster.PathCar[0])) || (areaMessage.Position == peerster.PathCar[1]) {
			//This saves people in our area
			peerster.areaMessageManagement(areaMessage, IPofCar)

		} else {

			peerster.PosCarsInArea.Mutex.Lock()

			for _, carInfo := range peerster.PosCarsInArea.Slice {
				if carInfo.IPCar == IPofCar {
					carInfo.Origin = areaMessage.Origin
					carInfo.Position = areaMessage.Position
				}
			}
			peerster.PosCarsInArea.Mutex.Unlock()
		}
		// This would trigger if is the car is stopped
	} else {
		if utils.AreaPositioner(areaMessage.Position) == utils.AreaPositioner(peerster.PathCar[0]) {
			peerster.areaMessageManagement(areaMessage, IPofCar)
		} else {

			peerster.PosCarsInArea.Mutex.Lock()

			for _, carInfo := range peerster.PosCarsInArea.Slice {
				if carInfo.IPCar == IPofCar {
					carInfo.Origin = areaMessage.Origin
					carInfo.Position = areaMessage.Position
				}
			}
			peerster.PosCarsInArea.Mutex.Unlock()
		}
	}

}
func (peerster *Peerster) areaMessageManagement(areaMessage *messaging.AreaMessage, IPofCar string) {
	carExists := false

	peerster.PosCarsInArea.Mutex.Lock()

	for _, carInfo := range peerster.PosCarsInArea.Slice {
		if carInfo.IPCar == IPofCar {
			carInfo.Origin = areaMessage.Origin
			carInfo.Position = areaMessage.Position
			carInfo.Channel <- false
			// carInfo.Channel = make(chan bool)
			carExists = true
			break
		}
	}
	peerster.PosCarsInArea.Mutex.Unlock()
	if carExists == false {
		origin := areaMessage.Origin
		position := areaMessage.Position
		peerster.SaveCarInAreaStructure(origin, position, IPofCar)
	}
}
func (peerster *Peerster) SaveCarInAreaStructure(origin string, position utils.Position, IPofCar string) {
	peerster.PosCarsInArea.Mutex.RLock()
	for _, car := range peerster.PosCarsInArea.Slice {
		if car.IPCar == IPofCar {
			peerster.PosCarsInArea.Mutex.RUnlock()
			return // car already exists //TODO this code is useless cus the check is done before calling the function
		}
	}
	//fmt.Println("Salvando chaval", IPofCar)
	peerster.PosCarsInArea.Mutex.RUnlock()

	infoOfCar := &utils.CarInformation{
		Origin:   origin,
		Position: position,
		Channel:  make(chan bool, 1),
		IPCar:    IPofCar,
		Active:   false,
	}
	peerster.PosCarsInArea.Mutex.Lock()
	peerster.PosCarsInArea.Slice = append(peerster.PosCarsInArea.Slice, infoOfCar)
	peerster.PosCarsInArea.Mutex.Unlock()
	go peerster.timeoutAreaMessage(infoOfCar)
}
func (peerster *Peerster) timeoutAreaMessage(infoOfCar *utils.CarInformation) {
	delete := true
	infoOfCar.Active = false
	for delete {
		select {
		// Delete car
		case deleteCar := <-infoOfCar.Channel:
			if deleteCar {
				delete = false
			} else {
				delete = true
			}
		case <-time.After(TIMEOUTCARS * time.Second):
			// infoOfCar.Channel <- true
			if !peerster.AreaChangeSession.Active {
				peerster.SendTrace(utils.MessageTrace{
					Type: utils.Other,
					Text: fmt.Sprintf("Forgetting car %v", infoOfCar.Origin),
				})
				delete = false
			}
		}
	}
	peerster.deleteCar(infoOfCar)

	// First message from that car
}
func (peerster *Peerster) deleteCar(infoOfCar *utils.CarInformation) {

	peerster.PosCarsInArea.Mutex.Lock()

	for index, carInfo := range peerster.PosCarsInArea.Slice {
		if carInfo.IPCar == infoOfCar.IPCar {
			peerster.PosCarsInArea.Slice = append(peerster.PosCarsInArea.Slice[:index], peerster.PosCarsInArea.Slice[index+1:]...)
			break
		}
	}
	peerster.PosCarsInArea.Mutex.Unlock()
}
func (peerster *Peerster) handleIncomingResolutionM(colisionMessage *messaging.ColisionResolution, addr string) {
	if colisionMessage == nil {
		return
	}
	// If he is asking for another position, we ignore him
	/*if (colisionMessage.Position != peerster.PathCar[0]) && (colisionMessage.Position.X != -1) {
		peerster.ColisionInfo.IPCar = addr
		peerster.ColisionInfo.CoinFlip = -1
		peerster.SendNegotiationMessage()
		return
	}*/
	// Adding to list of trusted
	if peerster.WT {
		_, alreadyKnown := peerster.PksOfTrustedCars[colisionMessage.Origin]
		if !alreadyKnown {
			peerster.TrustedCars = append(peerster.TrustedCars, colisionMessage.Origin)
			newPk, err := keyParser.ParsePKCS1PublicKey(colisionMessage.Pk)
			if err != nil {
				fmt.Println("ERROR PARSING", err)
			}
			peerster.PksOfTrustedCars[colisionMessage.Origin] = *newPk
			peerster.Signatures[colisionMessage.Origin] = colisionMessage.Signature
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Other,
				Text: "Car " + colisionMessage.Origin + " now trusts me",
			})
		}
	}

	//This means that this message is the response to our coin flip
	if peerster.ColisionInfo.CoinFlip != 0 {
		hisCoinFlip := colisionMessage.CoinResult
		peerster.colisionLogicManager(hisCoinFlip)
		//If we are here is because someone is colliding with us and send us his coin flip
	} else {
		// We answer him back with the coin flip
		coinFlip := peerster.NegotiationCoinflip()

		if peerster.AreaChangeSession.Active {
			// If we have an area change session active, we want to autowin the coinflip
			//peerster.AreaChangeSession.Channel <- true // we interrupt the area change session FAKE NEWS
			peerster.PosCarsInArea.Mutex.RLock()

			for _, v := range peerster.PosCarsInArea.Slice {
				if v.Origin == colisionMessage.Origin {
					// If the car sending the coinflip is in our area, we win the coinflip
					if utils.AreaPositioner(v.Position) == utils.AreaPositioner(peerster.PathCar[0]) {
						coinFlip = MaxCoinflip + 1
					}
				}
			}
			peerster.PosCarsInArea.Mutex.RUnlock()
		}
		peerster.ColisionInfo.IPCar = addr
		peerster.ColisionInfo.CoinFlip = coinFlip
		go peerster.DeleteCoinTimer()
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Crash,
			Text: fmt.Sprintf("Received conflict message, responding with coinflip %v", coinFlip),
		})
		peerster.SendNegotiationMessage()
		peerster.colisionLogicManager(colisionMessage.CoinResult)
	}

}
func (peerster *Peerster) DeleteCoinTimer() {
	time.Sleep(time.Duration(4) * time.Second)
	peerster.ColisionInfo.IPCar = ""
	peerster.ColisionInfo.CoinFlip = 0

}
func (peerster *Peerster) handleIncomingAccidentMessage(alertToPolice *messaging.PrivateMessage) {
	if alertToPolice == nil {
		return
	}
	if alertToPolice.AlertPoliceCar == nil {
		return
	}
	plainText, _ := rsa.DecryptPKCS1v15(randCrytpo.Reader, &peerster.Sk, *alertToPolice.AlertPoliceCar)
	// Dcoding the GossipPacket
	alertObject := &messaging.AlertPolice{}
	err := protobuf.Decode(plainText, alertObject)
	if err != nil {
		fmt.Println("COULD NOT DCODE!", err)
		return
	}
	// There has been an accident, so download file and go there
	file := FileBeingDownloaded{
		MetafileHash:   alertObject.Evidence,
		Channel:        make(chan messaging.DataReply),
		CurrentChunk:   0,
		Metafile:       nil,
		DownloadedData: nil,
		FileName:       "accident.jpg",
	}
	peerster.downloadData([]string{alertToPolice.Origin}, file)
	peerster.SendTrace(utils.MessageTrace{
		Type: utils.Crash,
		Text: fmt.Sprintf("Received a report of an accident from %s at position %v", alertObject.Origin, alertObject.Position),
	})
	//Change path
	// He spawns at the middle
	/*startPos := utils.Position{
		X: 4,
		Y: 4,
	}*/
	var obstructions []utils.Position
	peerster.PathCar = CreatePath(peerster.CarMap, peerster.PathCar[0], alertObject.Position, obstructions)
	// Now the police car should start moving
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
	objectBytes, _ := protobuf.Encode(&alert)
	ciphertext, _ := rsa.EncryptPKCS1v15(randCrytpo.Reader, &peerster.PolicePk, objectBytes)
	privateAlert := messaging.PrivateMessage{
		AlertPoliceCar: &ciphertext,
		Destination:    "police",
	}
	if peerster.Name != "police" {
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Police,
			Text: fmt.Sprintf("Detected an accident at %v, sending report to police", peerster.PathCar[0]),
		})
	} else {
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Police,
			Text: fmt.Sprintf("The city is safe, once again. I am the hero they need, not the one they deserve."),
		})
	}
	//privateAlert.AlertPolice= &alert
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
	//If its the last position is because he is parking there, so no announcements of it
	if len(peerster.PathCar) != 1 {
		//TODO: Publish the spot in the newsgroup and initiate a session if someone wants to ask for the spot
		go peerster.spotAssigner()
		currentPosition := peerster.PathCar[0]
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Parking,
			Text: fmt.Sprintf("Parking spot found at %v, %v ", currentPosition.X, currentPosition.Y),
		})
		peerster.SendFreeSpotMessage()
	}

}
func (peerster *Peerster) handleIncomingSpotWinnerMessage(winnerAssigment *messaging.PrivateMessage) {
	if winnerAssigment == nil {
		return
	}
	if *winnerAssigment == (messaging.PrivateMessage{}) {
		return
	}
	if winnerAssigment.SpotPublicationWinner == nil {
		return
	}
	if winnerAssigment.Destination == peerster.Name {
		//Your are the winner so you change your path to the spot
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Parking,
			Text: fmt.Sprintf("Won parking spot at %v, %v ", winnerAssigment.SpotPublicationWinner.Position.X, winnerAssigment.SpotPublicationWinner.Position.Y),
		})
		var obstructions []utils.Position
		peerster.PathCar = CreatePath(peerster.CarMap, peerster.PathCar[0], winnerAssigment.SpotPublicationWinner.Position, obstructions)
	}
}
func (peerster *Peerster) handleIncomingSpotRequestMessage(request *messaging.PrivateMessage) {
	if request == nil {
		return
	}
	if *request == (messaging.PrivateMessage{}) {
		return
	}
	if request.SpotPublicationRequest == nil {
		return
	}
	peerster.CarsInterestedSpot.Requests = append(peerster.CarsInterestedSpot.Requests, *request)
}
func (peerster *Peerster) spotAssigner() {
	peerster.CarsInterestedSpot.SaveSpots = true
	time.Sleep(time.Duration(TIMEOUTSPOTS) * time.Second)
	//Now we should have all the request, so we have to assign a winner of the spot
	carsWantingSpot := len(peerster.CarsInterestedSpot.Requests)
	if carsWantingSpot > 0 {
		min := 0
		max := carsWantingSpot - 1
		coinFlip := rand.Intn(max-min+1) + min

		//The winner is
		winnerSpot := peerster.CarsInterestedSpot.Requests[coinFlip]
		//Send private message to the winner
		spotPublicationWinner := messaging.SpotPublicationWinner{
			Position: winnerSpot.SpotPublicationRequest.Position,
		}
		privateAlert := messaging.PrivateMessage{
			Origin:                peerster.Name,
			HopLimit:              20,
			ID:                    0,
			Destination:           winnerSpot.Origin,
			SpotPublicationWinner: &spotPublicationWinner,
		}
		for _, private := range peerster.CarsInterestedSpot.Requests {
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Parking,
				Text: fmt.Sprintf("Car %s wants the spot", private.Origin),
			})
		}

		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Parking,
			Text: fmt.Sprintf("Decided parking spot winner: %s", winnerSpot.Origin),
		})
		peerster.sendNewPrivateMessage(privateAlert)
		peerster.CarsInterestedSpot.SaveSpots = false
		peerster.CarsInterestedSpot.Requests = nil
	}
}
func (peerster *Peerster) colisionLogicManager(hisCoinFlip int) {
	//If our coinflip is superior we don´t have to recalculate path
	if peerster.ColisionInfo.CoinFlip > hisCoinFlip {
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Crash,
			Text: fmt.Sprintf("Won conflict resolution coinflip. My coinflip: %v, their coinflip: %v", peerster.ColisionInfo.CoinFlip, hisCoinFlip),
		})
		peerster.Winner = true
		//This means that we have to move
	} else {
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Crash,
			Text: fmt.Sprintf("Lost conflict resolution coinflip. My coinflip: %v, their coinflip: %v", peerster.ColisionInfo.CoinFlip, hisCoinFlip),
		})
		isnil := true
		var carPathAux []utils.Position
		for isnil {
			isnil = false
			var obstructions []utils.Position
			// We tell the pathfinding alogirthm that we can't go there
			//obstructions = append(obstructions, peerster.PathCar[1])
			//TODO: EXPERIMENTAL
			//Also put as obsticles surrounding vehicles
			for _, car := range peerster.PosCarsInArea.Slice {
				if peerster.surroundingCar(car.Position) {
					obstructions = append(obstructions, car.Position)
				}
			}

			carPathAux = CreatePath(peerster.CarMap, peerster.PathCar[0], peerster.PathCar[len(peerster.PathCar)-1], obstructions)
			if (carPathAux == nil) || (len(carPathAux) == 0) {
				isnil = true
				time.Sleep(time.Duration(1) * time.Second)
			}
			// To avoid the path full of 0
			if (len(carPathAux) > 1) && ((carPathAux[0].X == 0) && (carPathAux[0].Y == 0)) {
				isnil = true
				time.Sleep(time.Duration(1) * time.Second)
			}
		}
		peerster.PathCar = carPathAux

	}
	//Reset colision object
	peerster.ColisionInfo.CoinFlip = 0
	peerster.ColisionInfo.NumberColisions = 0
	peerster.ColisionInfo.IPCar = ""
	if peerster.AreaChangeSession.Active {
		peerster.AreaChangeSession.Channel <- false
	}
}
func (peerster *Peerster) surroundingCar(carPos utils.Position) bool {
	ourX := peerster.PathCar[0].X
	ourY := peerster.PathCar[0].Y
	hisX := carPos.X
	hisY := carPos.Y
	if (ourX+1 == hisX) || (ourY == hisY) {
		return true
	}
	if (ourX-1 == hisX) || (ourY == hisY) {
		return true
	}
	if (ourY+1 == hisY) || (ourX == hisX) {
		return true
	}
	if (ourY-1 == hisY) || (ourX == hisX) {
		return true
	}
	return false
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
		if int(i) >= len(peerster.ReceivedMessages.Map[origin]) {
			break
		}
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
		//fmt.Printf("PRIVATE origin %s hop-limit %v contents %s \n", message.Origin, message.HopLimit, message.Text)
		peerster.addToPrivateMessages(*message)
		//peerster.handleIncomingAreaChangeResponse(*message.AreaChangeResponse)
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
		if i >= len(peerster.KnownPeers) {
			break
		}
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
	if originAddr.String() != utils.ServerAddress {
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
			peerster.handleIncomingAccidentMessage(receivedPacket.Private)
		}

		//Function where the cars that won receive the message
		peerster.handleIncomingSpotWinnerMessage(receivedPacket.Private)

		//Function where the spotter saves the cars asking for the spot
		peerster.handleIncomingSpotRequestMessage(receivedPacket.Private)

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

	} else {
		receivedPacket := &utils.ServerMessage{}
		protobuf.Decode(buffer, receivedPacket)

		//Function where the server tells the car if he has entered an accidented zone
		peerster.handleIncomingServerAccidentMessage(receivedPacket)

		//Function where the server tells the car if he has entered a zone with free spot
		peerster.handleIncomingServerSpotMessage(receivedPacket)
	}
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
