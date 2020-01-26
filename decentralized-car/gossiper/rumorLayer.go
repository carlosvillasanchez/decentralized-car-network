package gossiper

import (
	"fmt"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
	"sync"
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
	}
	rumorMessage := messaging.RumorMessage{
		Newsgroup:         "", //TODO get newsgroup
		AreaChangeMessage: &message,
		AccidentMessage:   nil,
	}
	peerster.sendNewRumorMessage(rumorMessage)
}

func (peerster *Peerster) handleIncomingAreaChange(message messaging.RumorMessage, originAddress string) {
	fmt.Printf("this guy %+v\n", message)
	if message.AreaChangeMessage == nil || message.Origin == peerster.Name {
		return
	}
	// Someone wants to move to a position.
	// Check if we are in that position. If we are, send an AreaChangeResponse back saying fuck off
	// If not, what do we do? anyway we add the ip to our known peers
	peerster.SaveCarInAreaStructure(message.Origin, utils.Position{}, originAddress)
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
	peerster.sendNewPrivateMessage(spotRequest)
}

func (peerster *Peerster) handleIncomingAccident(message messaging.RumorMessage) {
	if message.AccidentMessage == nil {
		return
	}
	// send to channel that is received by the moving goroutine?
	// should the moving goroutine be structured as having an "interrupt" channel that has a timeout, which continues
	// the loop? else it has to repath etc
}
