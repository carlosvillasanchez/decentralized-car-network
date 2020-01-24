package gossiper

import "github.com/tormey97/decentralized-car-network/decentralized-car/messaging"

func (peerster *Peerster) handleIncomingAreaChangeResponse(response messaging.AreaChangeResponse) {
	// guy just told us hes in our spot. what do we do? repath? leave it to the movement thread what to do

}
