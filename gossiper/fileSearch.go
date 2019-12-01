package gossiper

import (
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"github.com/tormey97/Peerster/utils"
	"net"
	"strings"
	"sync"
	"time"
)

const INITIAL_BUDGET = 2
const MATCH_THRESHOLD = 2

type FileSearch struct {
	Keywords        []string
	Budget          int
	BudgetSpecified bool
	MatchCount      int
}

type FileSearchSessions struct {
	Array []FileSearch
	Mutex sync.RWMutex
}

func (sessions *FileSearchSessions) AddSession(keywords []string, budget int) {
	found, _ := sessions.FindSession(keywords)
	if found { //TODO may not be necessary
		return
	}
	budgetSpecified := budget == 0
	if budgetSpecified {
		budget = INITIAL_BUDGET
	}
	session := FileSearch{
		Keywords:        keywords,
		Budget:          budget,
		BudgetSpecified: budgetSpecified,
		MatchCount:      0,
	}
	sessions.Mutex.Lock()
	defer sessions.Mutex.Unlock()
	sessions.Array = append(sessions.Array, session)
}

func (sessions *FileSearchSessions) RemoveSession(keywords []string) {
	found, i := sessions.FindSession(keywords)
	if !found {
		return
	}
	sessions.Mutex.Lock()
	defer sessions.Mutex.Unlock()
	sessions.Array = append(sessions.Array[:i], sessions.Array[i+1:]...)
}

func (sessions *FileSearchSessions) FindSession(keywords []string) (bool, int) {
	sessions.Mutex.RLock()
	defer sessions.Mutex.RUnlock()
	for i := range sessions.Array {
		session := sessions.Array[i]
		if messaging.SliceEqual(keywords, session.Keywords) {
			return true, i
		}
	}
	return false, 0
}

func (sessions *FileSearchSessions) FindMatchingSessions(reply messaging.SearchReply) {
	sessions.Mutex.RLock()
	for i := range sessions.Array {
		session := sessions.Array[i]
		matchedFilenames := []string{}
		for j := range reply.Results {
			result := reply.Results[j]
			for x := range session.Keywords {
				if strings.Contains(result.FileName, session.Keywords[x]) { // TODO need to use REGEXP here? or what?
					if !messaging.SliceContains(result.FileName, matchedFilenames) {
						matchedFilenames = append(matchedFilenames, result.FileName)
						sessions.Mutex.RUnlock()
						sessions.Mutex.Lock()
						session.MatchCount++
						sessions.Array[i] = session
						sessions.Mutex.Unlock()
						sessions.Mutex.RLock()
					}
					break
				}
			}
		}
	}
}

type FileMatches struct {
	Map   map[string][]messaging.SearchResult
	Mutex sync.RWMutex
}

func (fileMatches *FileMatches) isFullyMatched(filename string) bool {
	fileMatches.Mutex.RLock()
	defer fileMatches.Mutex.RUnlock()
	file, ok := fileMatches.Map[filename]
	if !ok {
		utils.DebugPrintln("Tried to check match for nonexistent file", filename)
		return false
	}
	foundChunks := []uint64{}
	for i := range file {
		result := file[i]
		for j := range result.ChunkMap {
			alreadyFound := false
			for x := range foundChunks {
				if foundChunks[x] == result.ChunkMap[j] {
					alreadyFound = true
					break
				}
			}
			if alreadyFound {
				break
			}
			foundChunks = append(foundChunks, result.ChunkMap[j])
		}
	}
	if len(foundChunks) == 99999999 { // TODO FILESIZE
		return true
	}
	return false
}

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

// Tries to add the search request to the list of recent search requests. If it's already in there, returns false and
// doesn't add it - otherwise, starts a 0.5 second timer that will remove it and returns true.
func (peerster *Peerster) addToRecentSearchRequests(request *messaging.SearchRequest) bool {
	peerster.RecentSearchRequests.Mutex.RLock()
	for i := range peerster.RecentSearchRequests.Array {
		recentRequest := peerster.RecentSearchRequests.Array[i]
		if recentRequest.Origin == request.Origin && messaging.SliceEqual(request.Keywords, recentRequest.Keywords) {
			peerster.RecentSearchRequests.Mutex.RUnlock()
			utils.DebugPrintln("Rejected search request")
			return false
		}
	}
	peerster.RecentSearchRequests.Mutex.RUnlock()
	peerster.RecentSearchRequests.Mutex.Lock()
	peerster.RecentSearchRequests.Array = append(peerster.RecentSearchRequests.Array, *request)
	peerster.RecentSearchRequests.Mutex.Unlock()
	go func() {
		time.Sleep(500 * time.Millisecond)
		peerster.RecentSearchRequests.Mutex.RLock()
		for i := range peerster.RecentSearchRequests.Array {
			recentRequest := peerster.RecentSearchRequests.Array[i]
			if recentRequest.Origin == request.Origin && messaging.SliceEqual(request.Keywords, recentRequest.Keywords) {
				peerster.RecentSearchRequests.Mutex.RUnlock()
				peerster.RecentSearchRequests.Mutex.Lock()
				peerster.RecentSearchRequests.Array = append(peerster.RecentSearchRequests.Array[:i], peerster.RecentSearchRequests.Array[i+1:]...)
				peerster.RecentSearchRequests.Mutex.Unlock()
				return
			}
		}
		peerster.RecentSearchRequests.Mutex.RUnlock()
	}()
	return true
}

func (peerster *Peerster) handleIncomingSearchRequest(request *messaging.SearchRequest, originAddr net.UDPAddr) {
	if request == nil || !peerster.addToRecentSearchRequests(request) {
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
	// We find the session that belongs to this reply, if any. Then, we should add a match to the session somehow. The match will simply be the searchreply.
	peerster.FileSearchSessions.FindMatchingSessions(*reply)

}
