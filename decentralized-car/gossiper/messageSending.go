package gossiper

import (
	"fmt"
	utils2 "github.com/tormey97/decentralized-car-network/utils"
	"golang.org/x/net/trace"
	"time"

	"github.com/dedis/protobuf"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
)

func (peerster *Peerster) BroadcastCarPosition() {
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

func (peerster *Peerster) SendTrace(trace utils2.MessageTrace) {
	packet := utils.ServerNodeMessage{
		Position: &peerster.PathCar[0],
	}
	peerAddr := utils.StringAddrToUDPAddr(utils.ServerAddress)
	packetBytes, _ := protobuf.Encode(&packet)
	peerster.Conn.WriteToUDP(packetBytes, &peerAddr)
}

func (peerster *Peerster) SendInfoToServer() {
	go func() {
		for {
			time.Sleep(time.Duration(peerster.BroadcastTimer) * time.Second)

		}
	}()
}
func (peerster *Peerster) SendPosToServer() {
	packet := utils.ServerNodeMessage{
		Position: &peerster.PathCar[0],
	}
	peerAddr := utils.StringAddrToUDPAddr(utils.ServerAddress)
	packetBytes, _ := protobuf.Encode(&packet)
	peerster.Conn.WriteToUDP(packetBytes, &peerAddr)
}

func (peerster *Peerster) SendNegotiationMessage() {
	var colisionMessage messaging.ColisionResolution
	if len(peerster.PathCar) != 1 {
		colisionMessage = messaging.ColisionResolution{
			Origin:     peerster.Name,
			CoinResult: peerster.ColisionInfo.CoinFlip,
			Position:   peerster.PathCar[1],
		}
		// If we are stopped we will send our current position
	} else {
		positionAux := utils.Position{
			X: -1,
			Y: -1,
		}
		colisionMessage = messaging.ColisionResolution{
			Origin:     peerster.Name,
			CoinResult: peerster.ColisionInfo.CoinFlip,
			Position:   positionAux,
		}
	}

	packet := messaging.GossipPacket{
		Colision: &colisionMessage,
	}
	var blacklist []string
	peerster.sendToPeer(peerster.ColisionInfo.IPCar, packet, blacklist)
}
