package gossiper

import (
	"github.com/tormey97/Peerster/messaging"
	"net"
)

func (peerster *Peerster) requestFile(destination string, metafileHash []byte) {
	request := messaging.DataRequest{
		Origin:      peerster.Name,
		Destination: destination,
		HopLimit:    11,
		HashValue:   metafileHash,
	}
	// TODO start a "download session", marked by the metafile hash + destination?
	peerster.handleIncomingDataRequest(&request, messaging.StringAddrToUDPAddr(peerster.GossipAddress))
}

func (peerster *Peerster) startFileDownload(peerIdentifier string, metafileHash string) {

}

func (peerster *Peerster) startFileUpload(metafileHash []byte) {

}

func (peerster *Peerster) handleIncomingDataReply(reply *messaging.DataReply, originAddr net.UDPAddr) {

}

func (peerster *Peerster) handleIncomingDataRequest(request *messaging.DataRequest, originAddr net.UDPAddr) {
	if request.Destination == peerster.Name {
		//TODO we see if we have the file, and then send it back
		file, ok := peerster.SharedFiles[string(request.HashValue)]
		if !ok {
			//TODO We don't have the file - how do we respond?
		}
		chunks, ok := peerster.FileChunks[string(request.HashValue)]
		if !ok {
			// TODO This looks like an impossible situation
		}

	} else if request.HopLimit == 0 {
		return
	} else {
		request.HopLimit--
		peerster.nextHopRoute(&messaging.GossipPacket{DataRequest: request}, request.Destination)
	}
}
