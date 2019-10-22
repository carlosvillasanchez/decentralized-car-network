package gossiper

import "time"

func (peerster *Peerster) addToNextHopTable(id, originAddr string) {
	hopTable := peerster.NextHopTable
	hopTable[id] = originAddr
	peerster.NextHopTable = hopTable
}

func (peerster *Peerster) SendRouteMessage() {
	peerster.sendNewRumorMessage("")
}

func (peerster *Peerster) SendRouteMessages() {
	if peerster.RTimer == 0 {
		return
	}
	go func() {
		time.Sleep(time.Duration(peerster.RTimer) * time.Second)
		peerster.SendRouteMessage()
	}()
}
