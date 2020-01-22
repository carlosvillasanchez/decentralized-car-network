package gossiper

import (
	"fmt"
	"time"

	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
)

func (peerster *Peerster) BroadcastCarPosition() {
	go func() {
		for {
			time.Sleep(time.Duration(peerster.BroadcastTimer) * time.Second)
			areaMessage := messaging.AreaMessage{
				Origin:   peerster.Name,
				Position: peerster.PathCar[0],
			}
			packet := messaging.GossipPacket{
				Area: &areaMessage,
			}
			var blacklist []string
			for i := range peerster.KnownPeers {
				peer := peerster.KnownPeers[i]
				err := peerster.sendToPeer(peer, packet, blacklist)
				if err != nil {
					fmt.Printf("Could not send to peer %q, reason: %s \n", peer, err)
				}
			}
		}
	}()
}
