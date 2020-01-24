package gossiper

import (
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
	"sync"
)

type AreaChangeSession struct {
	sync.RWMutex
	utils.Position
	Channel        chan messaging.AreaChangeResponse
	Active         bool
	CollisionCount int
}

func (peerster *Peerster) sendAreaChangeMessage(pos utils.Position) {
	message := messaging.AreaChangeMessage{
		Position: pos,
	}
	rumorMessage := messaging.RumorMessage{
		Newsgroup:         "", //TODO get newsgroup
		AreaChangeMessage: &message,
		AccidentMessage:   nil,
	}
	peerster.sendNewRumorMessage(rumorMessage)
	peerster.handleIncomingRumor(&rumorMessage, utils.StringAddrToUDPAddr(peerster.GossipAddress), false)
}

func (peerster *Peerster) handleIncomingAreaChange(message messaging.RumorMessage, originAddress string) {
	if message.AreaChangeMessage == nil {
		return
	}
	// Someone wants to move to a position.
	// Check if we are in that position. If we are, send an AreaChangeResponse back saying fuck off
	// If not, what do we do? anyway we add the ip to our known peers
	peerster.addToKnownPeers(originAddress)
	if peerster.PathCar[0] == message.AreaChangeMessage.Position {
		privateMessage := messaging.PrivateMessage{
			Destination:        message.Origin,
			AreaChangeResponse: messaging.AreaChangeResponse{},
		}
		peerster.sendNewPrivateMessage(privateMessage)
	}
}

func (peerster *Peerster) handleIncomingAccident(message messaging.RumorMessage) {
	if message.AccidentMessage == nil {
		return
	}
	// send to channel that is received by the moving goroutine?
	// should the moving goroutine be structured as having an "interrupt" channel that has a timeout, which continues
	// the loop? else it has to repath etc
}
