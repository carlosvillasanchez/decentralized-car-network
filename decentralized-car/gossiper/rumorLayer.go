package gossiper

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
	"crypto"
	"crypto/rsa"
)

type AreaChangeSession struct {
	sync.RWMutex
	utils.Position
	Channel        chan bool
	Active         bool
	CollisionCount int
}

func (peerster *Peerster) sendAreaChangeMessage(pos utils.Position) {
	message := messaging.AreaChangeMessage{
		NextPosition:    pos,
		CurrentPosition: peerster.PathCar[0],
		IpofCarChanging: peerster.GossipAddress,
	}
	rumorMessage := messaging.RumorMessage{
		Newsgroup:         strconv.Itoa(utils.AreaPositioner(peerster.PathCar[1])), //TODO get newsgroup
		AreaChangeMessage: &message,
		AccidentMessage:   nil,
	}
	peerster.sendNewRumorMessage(rumorMessage)
}
func (peerster *Peerster) SendFreeSpotMessage() {
	message := messaging.SpotPublishMessage{
		Position: peerster.PathCar[0],
		Signatures: peerster.Signatures,
		IP: peerster.GossipAddress,
	}
	rumorMessage := messaging.RumorMessage{
		SpotPublishMessage: &message,
		Newsgroup:          ParkingNewsGroup, //TODO get newsgroup

	}
	peerster.sendNewRumorMessage(rumorMessage)
}

func (peerster *Peerster) handleIncomingAreaChange(message messaging.RumorMessage) {
	if message.AreaChangeMessage == nil || message.Origin == peerster.Name {
		return
	}
	// Someone wants to move to a position.
	// Check if we are in that position. If we are, send an AreaChangeResponse back saying fuck off
	// If not, what do we do? anyway we add the ip to our known peers
	peerster.SaveCarInAreaStructure(message.Origin, message.AreaChangeMessage.CurrentPosition, message.AreaChangeMessage.IpofCarChanging)
	for _, v := range peerster.PosCarsInArea.Slice {
		fmt.Printf("POS CARS IN AREA:  %+v \n", v)
	}
	/*
		This code sends a specific response if there is a conflict, but I think that's not actually necessary.
		if peerster.PathCar[0] == message.AreaChangeMessage.NextPositionPosition {
			privateMessage := messaging.PrivateMessage{
				Destination:        message.Origin,
				AreaChangeResponse: &messaging.AreaChangeResponse{},
			}
			peerster.sendNewPrivateMessage(privateMessage)
		}*/
}
func (peerster *Peerster) handleIncomingFreeSpotMessage(message messaging.RumorMessage) {
	if message.SpotPublishMessage == nil {
		return
	}
	//To avoid requesting the spot two times
	if peerster.PostulatedAlready == true {
		return
	}
	if peerster.WT {
		verified := false
		for _, trustedCar := range peerster.TrustedCars {
			signature, ok := message.SpotPublishMessage.Signatures[trustedCar]
			if ok {
				pkToVerify := peerster.PksOfTrustedCars[trustedCar]
				newhash := crypto.SHA256
				toSing := []byte(message.SpotPublishMessage.IP)
				pssh := newhash.New()
				pssh.Write(toSing)
				hashed := pssh.Sum(nil)
				err := rsa.VerifyPKCS1v15(&pkToVerify, newhash, hashed, signature)
				if err == nil {
					verified = true
					peerster.SendTrace(utils.MessageTrace{
						Type: utils.Parking,
						Text: "Parking spot announcer (" + message.SpotPublishMessage.IP + ") verified with car " + trustedCar,
					})
					break
				}
			}
		}
		if verified == false {
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Parking,
				Text: "Parking spot announcer (" + message.SpotPublishMessage.IP + ") not verified",
			})
			return
		}
	}
	fmt.Println("DDDDDDDDDDDDDDDDD")
	request := messaging.SpotPublicationRequest{
		Position: message.SpotPublishMessage.Position,
	}

	//Request the spot
	spotRequest := messaging.PrivateMessage{
		Origin:                 peerster.Name,
		HopLimit:               20,
		ID:                     0,
		Destination:            message.Origin,
		SpotPublicationRequest: &request,
	}
	fmt.Println("GGGGGGGGGGGGG")
	fmt.Printf("%v \n", request)
	fmt.Printf("%v \n", spotRequest.Destination)
	peerster.sendNewPrivateMessage(spotRequest)
	peerster.PostulatedAlready = true
}

func (peerster *Peerster) handleIncomingAccident(message messaging.RumorMessage) {
	if message.AccidentMessage == nil {
		return
	}
	// send to channel that is received by the moving goroutine?
	// should the moving goroutine be structured as having an "interrupt" channel that has a timeout, which continues
	// the loop? else it has to repath etc
}
