package gossiper

import (
	"fmt"
	"time"

	"github.com/dedis/protobuf"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
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

			peerster.PosCarsInArea.Mutex.RLock()
			for i := range peerster.PosCarsInArea.Slice {
				peer := peerster.PosCarsInArea.Slice[i].IPCar
				err := peerster.sendToPeer(peer, packet, blacklist)
				if err != nil {
					fmt.Printf("Could not send to peer %q, reason: %s \n", peer, err)
				}
			}
			peerster.PosCarsInArea.Mutex.RUnlock()
		}
	}()
}
func (peerster *Peerster) SendInfoToServer() {
	go func() {
		for {
			time.Sleep(time.Duration(peerster.BroadcastTimer) * time.Second)

			packet := utils.ServerNodeMessage{
				Position: &peerster.PathCar[0],
			}
			peerAddr := utils.StringAddrToUDPAddr(utils.ServerAddress)
			packetBytes, _ := protobuf.Encode(&packet)
			peerster.Conn.WriteToUDP(packetBytes, &peerAddr)
		}
	}()
}
func (peerster *Peerster) SendNegotiationMessage() {
	colisionMessage := messaging.ColisionResolution{
		Origin:     peerster.Name,
		CoinResult: peerster.ColisionInfo.CoinFlip,
	}
	packet := messaging.GossipPacket{
		Colision: &colisionMessage,
	}
	var blacklist []string
	peerster.sendToPeer(peerster.ColisionInfo.IPCar, packet, blacklist)
}
