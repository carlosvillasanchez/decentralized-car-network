package gossiper

import (
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
)

func (peerster *Peerster) sendAreaChangeMessage(pos utils.Position) {
	message := messaging.AreaChangeMessage{
		Position: pos,
	}
	rumorMessage := messaging.RumorMessage{
		Origin:            peerster.Name,
		ID:                peerster.MsgSeqNumber,
		Text:              "",
		Newsgroup:         "", //TODO get newsgroup
		AreaChangeMessage: &message,
		AccidentMessage:   nil,
	}
	peerster.MsgSeqNumber += 1
	peerster.handleIncomingRumor(&rumorMessage, utils.StringAddrToUDPAddr(peerster.GossipAddress), false)
}

func (peerster *Peerster) handleIncomingAreaChange(message messaging.RumorMessage) {
	if message.AreaChangeMessage == nil {
		return
	}
	// Someone wants to move to a position.
	// Check if we are in that position. If we are, send an AreaChangeResponse back saying fuck off
	// If not, what do we do? nothing!
	if peerster.PathCar[0] == message.AreaChangeMessage.Position {
		privateMessage := messaging.PrivateMessage{
			Destination:        message.Origin,
			AreaChangeResponse: &messaging.AreaChangeResponse{},
		}
		peerster.sendNewPrivateMessage(privateMessage)
	}
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
