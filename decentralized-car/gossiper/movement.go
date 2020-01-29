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

	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/tormey97/decentralized-car-network/utils"
)

const MaxCoinflip = 7000

func (peerster *Peerster) isLastPosition() bool {
	if len(peerster.PathCar) <= 1 {
		//TODO: CarPath( change the path to a random one
		return true
	}
	return false
}
func (peerster *Peerster) MoveCarPosition() {

	go func() {
		for {
			// fmt.Println(peerster.Newsgroups)
			// fmt.Println(peerster.PathCar)
			// If it is a police car stopped don't do anything
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Crash,
				Text: fmt.Sprintf("This is a trace 1: %v", peerster.Name),
			})
			meanTime := MovementTimer * 1000
			randomness := rand.Intn(500)
			time.Sleep(time.Duration(meanTime+randomness) * time.Millisecond) //TODO moved this out of the if, si that ok?
			for _, value := range peerster.PosCarsInArea.Slice {
				peerster.SendTrace(utils.MessageTrace{
					Type: utils.Crash,
					Text: fmt.Sprintf("This is a trace 2: %v", peerster.Name),
				})

				peerster.SendTrace(utils.MessageTrace{
					Type: utils.Other,
					Text: fmt.Sprintf("Car %v: knowing %v", peerster.Name, value),
				})
			}
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Crash,
				Text: fmt.Sprintf("This is a trace 3: %v", peerster.Name),
			})
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Other,
				Text: fmt.Sprintf("-------------------------------------------"),
			})
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Other,
				Text: fmt.Sprintf("%v", peerster.PathCar),
			})
			if peerster.PathCar != nil && !peerster.isLastPosition() {
				peerster.SendTrace(utils.MessageTrace{
					Type: utils.Crash,
					Text: fmt.Sprintf("This is a trace 4: %v", peerster.Name),
				})
				areaChange := peerster.changeOfArea()
				//There is a change in the area zone, so different procedure
				if areaChange {
					peerster.SendTrace(utils.MessageTrace{
						Type: utils.Crash,
						Text: fmt.Sprintf("This is a trace 5: %v", peerster.Name),
					})
					if peerster.Winner {
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 6123123: %v", peerster.Name),
						})
						peerster.Winner = false
						peerster.positionAdvancer()
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 6: %v", peerster.Name),
						})

						// fmt.Println(peerster.PathCar)
						// time.Sleep(time.Duration(MovementTimer) * time.Second)
						// peerster.PathCar = peerster.PathCar[1:]
						// peerster.BroadcastCarPosition()
						continue
					}
					if !peerster.AreaChangeSession.Active {
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 612412121212211221: %v", peerster.Name),
						})
						peerster.SendAreaChangeMessage(peerster.PathCar[1])
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Other,
							Text: fmt.Sprintf("Starting area change, target location: %v", peerster.PathCar[1]),
						})
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 812: %v", peerster.Name),
						})
						peerster.startAreaChangeSession()
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 7: %v", peerster.Name),
						})

						continue
					}
					peerster.SendTrace(utils.MessageTrace{
						Type: utils.Crash,
						Text: fmt.Sprintf("This is a trace 123: %v", peerster.Name),
					})
					peerster.collisionChecker()
					peerster.SendTrace(utils.MessageTrace{
						Type: utils.Crash,
						Text: fmt.Sprintf("This is a trace 8: %v", peerster.Name),
					})
					if peerster.ColisionInfo.NumberColisions >= 2 {
						peerster.SendTrace(utils.MessageTrace{
							Type: utils.Crash,
							Text: fmt.Sprintf("This is a trace 9: %v", peerster.Name),
						})
						// peerster.AreaChangeSession.Channel <- false
						peerster.negotationOfColision()
					}
				} else {
					peerster.SendTrace(utils.MessageTrace{
						Type: utils.Crash,
						Text: fmt.Sprintf("This is a trace 10: %v", peerster.Name),
					})
					//This function will advance the car to the next position if possible, checking there are not other cars
					peerster.positionAdvancer()
				}
			}
		}
	}()
}

func (peerster *Peerster) startAreaChangeSession() {
	peerster.AreaChangeSession.Position = peerster.PathCar[1]
	peerster.AreaChangeSession.Active = true
	//for {
	select {
	case value := <-peerster.AreaChangeSession.Channel:
		if value {
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Other,
				Text: fmt.Sprintf("AAAAAAAAAAAAAAAAAAA"),
			})
			return
		} else {
			peerster.AreaChangeSession.Active = false
			peerster.SendTrace(utils.MessageTrace{
				Type: utils.Other,
				Text: fmt.Sprintf("BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"),
			})
		}
	case <-time.After(AreaChangeTimer * time.Second):
		peerster.SendTrace(utils.MessageTrace{
			Type: utils.Other,
			Text: fmt.Sprintf("CCCCCCCCCCCCCCCCCCCCCCCCCCCCC"),
		})
		peerster.UnsubscribeFromNewsgroup(strconv.Itoa(utils.AreaPositioner(peerster.PathCar[0])))
		peerster.SubscribeToNewsgroup(strconv.Itoa(utils.AreaPositioner(peerster.PathCar[1])))
		peerster.positionAdvancer()
		peerster.AreaChangeSession.Active = false
	}
	//}
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
		//If the car is changing from area subscribe to the new one
		if utils.AreaPositioner(peerster.PathCar[0]) != utils.AreaPositioner(peerster.PathCar[1]) {
			peerster.UnsubscribeFromNewsgroup(strconv.Itoa(utils.AreaPositioner(peerster.PathCar[0])))
			peerster.SubscribeToNewsgroup(strconv.Itoa(utils.AreaPositioner(peerster.PathCar[1])))
		}
		peerster.PathCar = peerster.PathCar[1:]
		fmt.Println(peerster.PathCar)
		peerster.BroadcastCarPosition()
		peerster.SendPosToServer()
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
	peerster.SendTrace(utils.MessageTrace{
		Type: utils.Crash,
		Text: fmt.Sprintf("This is a trace 11: %v", peerster.Name),
	})
	peerster.PosCarsInArea.Mutex.Lock()
	peerster.SendTrace(utils.MessageTrace{
		Type: utils.Crash,
		Text: fmt.Sprintf("This is a trace 12: %v", peerster.Name),
	})
	defer peerster.PosCarsInArea.Mutex.Unlock()
	for _, carInfo := range peerster.PosCarsInArea.Slice {
		//If a car is in the position we want to move to, there is a collision
		if peerster.PathCar[1] == carInfo.Position {
			peerster.ColisionInfo.NumberColisions = peerster.ColisionInfo.NumberColisions + 1
			peerster.ColisionInfo.IPCar = carInfo.IPCar
			fmt.Println("-------------------------")
			fmt.Println(peerster.ColisionInfo.NumberColisions)
			fmt.Printf("CAR INFO: %+v \n", carInfo)
			fmt.Println(peerster.ColisionInfo.IPCar)
			return true
		}
	}
	peerster.ColisionInfo.NumberColisions = 0
	peerster.ColisionInfo.IPCar = ""
	peerster.ColisionInfo.CoinFlip = 0
	return false
}

func (peerster *Peerster) NegotiationCoinflip() int {
	//If he is standing still he should always win, he is not trying to move
	if len(peerster.PathCar) == 1 {
		return MaxCoinflip + 1
	}
	min := 1
	max := MaxCoinflip
	return rand.Intn(max-min+1) + min
}
func (peerster *Peerster) negotationOfColision() {
	// You flip a coin and send the information to the other guy
	//TODO: We have to add that if you are trying to change area,
	// and another guy from your current area wants to negotiate with you, you always win and stay still
	coinFlip := peerster.NegotiationCoinflip()
	peerster.ColisionInfo.CoinFlip = coinFlip
	go peerster.DeleteCoinTimer()
	peerster.SendTrace(utils.MessageTrace{
		Type: utils.Crash,
		Text: fmt.Sprintf("Detected conflict. Sending negotiation message with coinflip %v", coinFlip),
	})
	peerster.SendNegotiationMessage()

}
