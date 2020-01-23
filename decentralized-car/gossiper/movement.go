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
	"github.com/tormey97/decentralized-car-network/utils"
)

func (peerster *Peerster) MoveCarPosition() {
	areaChange := peerster.changeOfArea()
	//There is a change in the area zone, so different procedure
	if areaChange {

		//There is no change, so just move to position and broadcast
	} else {
		//This function will advance the car to the next position if possible, checking there are not other cars
		peerster.positionAdvancer()

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
		// x, a = peerster.PathCar[0], a[1:]
		peerster.PathCar = peerster.PathCar[1:]
		// There is a colision, do something
	} else {

	}
}
func (peerster *Peerster) collisionChecker() bool {
	for _, carInfo := range peerster.PosCarsInArea {
		//If a car is in the position we want to move to, there is a collision
		if peerster.PathCar[1] == carInfo.Position {
			return true
		}
	}
	return false
}
