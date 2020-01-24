package gossiper

import (
	// 	"errors"
	// 	"fmt"
	// 	"log"
	// 	"math/rand"
	// 	"net"
	// 	"strconv"
	// 	"sync"
	// 	"time"

	// 	"github.com/dedis/protobuf"
	// 	"github.com/tormey97/decentralized-car-network/decentralized-car/messaging"

	"math/rand"
	"time"

	"github.com/tormey97/decentralized-car-network/utils"
)

func (peerster *Peerster) MoveCarPosition() {

	go func() {
		for {
			time.Sleep(time.Duration(peerster.BroadcastTimer) * time.Second)
			areaChange := peerster.changeOfArea()
			//There is a change in the area zone, so different procedure
			if areaChange {
				peerster.sendAreaChangeMessage(peerster.PathCar[1])
				peerster.startAreaChangeSession()
				// we send an area change message, then we have to wait for a response
				// use a channel, set a timeout to
				// when moving into an area the car will stand still, so if another car begins negotiating with you
				// you will get the maximum coinflip and keep your spot.
				// to move into anothre area: send area change msg, create sessoin, wait for response, if response:
				// repath
				// else drive into slot
				//There is no change, so just move to position and broadcast
			} else {
				//This function will advance the car to the next position if possible, checking there are not other cars
				peerster.positionAdvancer()
			}
		}
	}()
}

func (peerster *Peerster) startAreaChangeSession() {
	peerster.AreaChangeSession.Position = peerster.PathCar[1]
	peerster.AreaChangeSession.Active = true
	for {
		select {
		case <-peerster.AreaChangeSession.Channel:
			if peerster.AreaChangeSession.CollisionCount == 0 {
				peerster.AreaChangeSession.CollisionCount++
				peerster.sendAreaChangeMessage(peerster.PathCar[1])
				// move to area
				// the other cars will add us to their known peers
				// start broadcasting positions, so we use positions to know who is where
				// we do normal collision negotiation when we try to move into a spot

			} else {
				// conflict twice, so we start negotiating with the guy
			}

			// we got a complaint, so wait one turn?
		case <-time.After(6 * time.Second):
			// move
			break
		}
	}
}

func (peerster *Peerster) changeOfArea() bool {
	// If the area we are into is different to the one we are going, changing area
	if utils.AreaPositioner(peerster.PathCar[0]) != utils.AreaPositioner(peerster.PathCar[1]) {
		return true
	}
	return false
}
func (peerster *Peerster) positionAdvancer() {
	if peerster.collisionChecker() == false {
		peerster.PathCar = peerster.PathCar[1:]

		// There is a colision, do something
	} else {
		//If there has been more than 2 colision, negotiate
		if peerster.ColisionInfo.NumberColisions >= 2 {
			peerster.negotationOfColision()
			// If not just wait without moving to see if something changes
		} else {
			return
		}

	}
}
func (peerster *Peerster) collisionChecker() bool {
	peerster.PosCarsInArea.Mutex.Lock()
	defer peerster.PosCarsInArea.Mutex.Unlock()
	for _, carInfo := range peerster.PosCarsInArea.Slice {
		//If a car is in the position we want to move to, there is a collision
		if peerster.PathCar[1] == carInfo.Position {
			peerster.ColisionInfo.NumberColisions = peerster.ColisionInfo.NumberColisions + 1
			peerster.ColisionInfo.IPCar = carInfo.IPCar
			return true
		}
	}
	peerster.ColisionInfo.NumberColisions = 0
	peerster.ColisionInfo.IPCar = ""
	peerster.ColisionInfo.CoinFlip = 0
	return false
}

func negotiationCoinflip() int {

	min := 1
	max := 7000
	return rand.Intn(max-min+1) + min
}
func (peerster *Peerster) negotationOfColision() {
	// You flip a coin and send the information to the other guy

	coinFlip := negotiationCoinflip()
	peerster.ColisionInfo.CoinFlip = coinFlip
	peerster.SendNegotiationMessage()

}
