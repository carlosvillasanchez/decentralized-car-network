package gossiper

import (
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"net"
	"strings"
)

// Sends a request to search for files in other nodes with keywords
func (peerster *Peerster) searchForFiles(keywords []string, budget int) {
	request := messaging.SearchRequest{
		Origin:   peerster.Name,
		Budget:   uint64(budget),
		Keywords: keywords,
	}
	packet := messaging.GossipPacket{SearchRequest: &request}
	_, err := peerster.sendToRandomPeer(packet, []string{})
	if err != nil {
		fmt.Printf("Couldn't send SearchRequest to random peer, reason: %s \n", err)
	}
}

func (peerster *Peerster) handleIncomingSearchRequest(request *messaging.SearchRequest, originAddr net.UDPAddr) {
	if request == nil {
		return
	}
	// TODO handle budget
	files := peerster.searchInLocalFiles(request.Keywords)
	reply := messaging.SearchReply{
		Origin:      peerster.Name,
		Destination: request.Origin,
		HopLimit:    10,
		Results:     peerster.createSearchResults(files),
	}
	fmt.Println(reply.Results)
	packet := messaging.GossipPacket{
		SearchReply: &reply,
	}
	peerster.nextHopRoute(&packet, request.Origin)
	peerster.distributeSearchRequest(request)
}

// Distributes the search request evenly among the peerster's neighbors using the Budget value.
// TODO make sure your definition of neighbor is correct (could be nexthop table with hop distance 1)
func (peerster *Peerster) distributeSearchRequest(request *messaging.SearchRequest) {
	request.Budget--
	knownPeers := peerster.KnownPeers
	packetsToSend := map[string]messaging.GossipPacket{}
	if request.Budget <= 0 {
		return
	}
	for request.Budget > 0 {
		for i := range knownPeers {
			request.Budget--
			packet, ok := packetsToSend[knownPeers[i]]
			if !ok {
				newRequest := request
				newRequest.Budget = 1
				packet = messaging.GossipPacket{SearchRequest: newRequest}
				packetsToSend[knownPeers[i]] = packet
			} else {
				packet.SearchRequest.Budget++
				packetsToSend[knownPeers[i]] = packet
			}
			if request.Budget == 0 {
				break
			}
		}
	}
	for i, v := range packetsToSend {
		err := peerster.sendToPeer(i, v, []string{})
		if err != nil {
			fmt.Printf("Couldn't distribute searchreply, reason: %s \n", err)
		}
	}

}

// Searches in the peerster's local shared files for filenames with the specified keywords
func (peerster *Peerster) searchInLocalFiles(keywords []string) []SharedFile {
	peerster.SharedFiles.Mutex.RLock()
	defer peerster.SharedFiles.Mutex.RUnlock()
	foundFiles := []SharedFile{}
	for i := range peerster.SharedFiles.Map {
		file := peerster.SharedFiles.Map[i]
		for j := range keywords {
			if strings.Contains(file.FileName, keywords[j]) { // TODO need to use REGEXP here? or what?
				foundFiles = append(foundFiles, file)
				break
			}
		}
	}
	return foundFiles
}

// Finds the indexes of the chunks that the peerster has stored locally of a specified file
func (peerster *Peerster) findChunksOfFile(metafileHash []byte) []uint64 {
	peerster.SharedFiles.Mutex.RLock()
	metafile := peerster.SharedFiles.Map[string(metafileHash)].Metafile
	peerster.SharedFiles.Mutex.RUnlock()
	foundChunks := []uint64{}
	for i := 0; i < len(metafile)/HashSize; i++ {
		lowerBound := i * HashSize
		upperBound := (i + 1) * HashSize
		if upperBound > len(metafile) {
			upperBound = len(metafile)
		}
		chunkHash := metafile[lowerBound:upperBound]
		peerster.FileChunks.Mutex.RLock()
		_, ok := peerster.FileChunks.Map[string(chunkHash)]
		peerster.FileChunks.Mutex.RUnlock()
		if ok {
			foundChunks = append(foundChunks, uint64(i))
		}
	}
	return foundChunks
}

// Converts a list of SharedFile to a list of SearchResult (so it has the right format for the GossipPacket)
func (peerster *Peerster) createSearchResults(foundFiles []SharedFile) []*messaging.SearchResult {
	results := []*messaging.SearchResult{}
	for i := range foundFiles {
		file := foundFiles[i]
		foundChunks := peerster.findChunksOfFile(file.MetafileHash)
		result := messaging.SearchResult{
			FileName:     file.FileName,
			MetafileHash: file.MetafileHash,
			ChunkMap:     foundChunks,
			ChunkCount:   uint64(len(file.Metafile) / HashSize),
		}
		results = append(results, &result)
	}
	return results
}

func (peerster *Peerster) handleIncomingSearchReply(reply *messaging.SearchReply, originAddr net.UDPAddr) {

}
