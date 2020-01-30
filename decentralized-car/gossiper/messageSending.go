package gossiper

import (
	"fmt"
	"time"

	utils2 "github.com/tormey97/decentralized-car-network/utils"

	"github.com/dedis/protobuf"
	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"
	"github.com/tormey97/decentralized-car-network/utils"
	"crypto/rsa"
	keyParser "crypto/x509"
	"crypto"
	"crypto/rand"
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
	trace.Text = fmt.Sprintf("Car %v: ", peerster.Name) + trace.Text
	packet := utils.ServerNodeMessage{
		Position: nil,
		Trace:    &trace,
	}
	addr := utils.StringAddrToUDPAddr(utils.ServerAddress)
	packetBytes, _ := protobuf.Encode(&packet)
	peerster.Conn.WriteToUDP(packetBytes, &addr)
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
	var pkParsed []byte
	var signature []byte
	if peerster.WT {
		toSing := []byte(peerster.ColisionInfo.IPCar)
		newhash := crypto.SHA256
		pssh := newhash.New()
		pssh.Write(toSing)
		hashed := pssh.Sum(nil)
		signature, _ = rsa.SignPKCS1v15(rand.Reader, &peerster.Sk, newhash, hashed) 
		pkParsed = keyParser.MarshalPKCS1PublicKey(&peerster.Pk)
	}
	if len(peerster.PathCar) != 1 {
		colisionMessage = messaging.ColisionResolution{
			Origin:     peerster.Name,
			CoinResult: peerster.ColisionInfo.CoinFlip,
			Position:   peerster.PathCar[1],
			Pk: 		pkParsed,
			Signature:  signature,
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
			Pk: 		pkParsed,
			Signature:  signature,
		}
	}
	/*if peerster.WT {
		toSing := []byte(peerster.ColisionInfo.IPCar)
		newhash := crypto.SHA256
		pssh := newhash.New()
		pssh.Write(toSing)
		hashed := pssh.Sum(nil)
		signature, err := rsa.SignPKCS1v15(rand.Reader, &peerster.Sk, newhash, hashed) 
		if err != nil {
			fmt.Println("ERROR SIGNING", err)
		}
		colisionMessage.Pk = peerster.Pk
		//colisionMessage.Signature = signature
	}*/

	packet := messaging.GossipPacket{
		Colision: &colisionMessage,
	}
	var blacklist []string
	peerster.sendToPeer(peerster.ColisionInfo.IPCar, packet, blacklist)
}
