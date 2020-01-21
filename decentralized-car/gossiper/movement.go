package gossiper

// import (
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
// 	"github.com/tormey97/decentralized-car-network/utils"
// )
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
	return false
}
func (peerster *Peerster) positionAdvancer() {

}
