package gossiper

import (
	"errors"
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"github.com/tormey97/Peerster/utils"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const INITIAL_BUDGET = 2
const MATCH_THRESHOLD = 2
const BUDGET_THRESHOLD = 32

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

// Finds the active search sessions for which there is a match. If there is a match, we should check
// if the threshold has been reached - if so we stop, if not we keep searching? Or not?
// AH only if the session did not have a specified budget.
func (sessions *FileSearchSessions) FindMatchingSessions(reply messaging.SearchReply) []int {
	sessions.Mutex.RLock()
	foundSessions := []int{}
	for i := range sessions.Array {
		session := sessions.Array[i]
		matchedFilenames := []string{}
		for j := range reply.Results {
			result := reply.Results[j]
			for x := range session.Keywords {
				if strings.Contains(result.FileName, session.Keywords[x]) { // TODO need to use REGEXP here? or what?
					if !messaging.SliceContains(result.FileName, matchedFilenames) {
						matchedFilenames = append(matchedFilenames, result.FileName)
					}
					break
				}
			}
		}
	}
	return foundSessions
}

type FileMatch struct {
	messaging.SearchResult
	Origin string
}

type FileMatches struct {
	Map   map[string][]FileMatch
	Mutex sync.RWMutex
}

func (fileMatches *FileMatches) addResults(reply messaging.SearchReply) {
	results := reply.Results
	for i := range results {
		result := results[i]
		hash := result.MetafileHash
		chunkString := ""
		for j := range result.ChunkMap {
			chunkString += strconv.Itoa(int(result.ChunkMap[j]))
			if j < len(result.ChunkMap)-1 {
				chunkString += ","
			}
		}
		fmt.Printf("FOUND match %s at %s metafile=%s chunks=%s \n", result.FileName, reply.Origin, result.MetafileHash, chunkString) //TODO arguments
		fileMatches.Mutex.RLock()
		_, ok := fileMatches.Map[string(hash)]
		fileMatches.Mutex.RUnlock()
		fileMatches.Mutex.Lock()
		utils.DebugPrintln(string(hash), "WHAT")
		if !ok {
			fileMatches.Map[string(hash)] = []FileMatch{{
				SearchResult: *result,
				Origin:       reply.Origin,
			}}
		} else {
			fileMatches.Map[string(hash)] = append(fileMatches.Map[string(hash)], FileMatch{
				SearchResult: *result,
				Origin:       reply.Origin,
			})
		}
		utils.DebugPrintln("FILEMATCHES MAP: ", fileMatches.Map[string(hash)][0])
		fileMatches.Mutex.Unlock()
	}
}

func (fileMatches *FileMatches) createDownloadChain(metafileHash []byte) ([]string, error) {
	if !fileMatches.isFullyMatched(metafileHash) {
		return nil, errors.New("file wasn't fully matched")
	}
	fileMatches.Mutex.RLock()
	results := fileMatches.Map[string(metafileHash)]
	fileMatches.Mutex.RUnlock()
	chunkCount := results[0].ChunkCount
	order := []string{}
	for i := 0; i < int(chunkCount); i++ {
		utils.DebugPrintln(i, "CHUNKI")

		// Find all nodes that have this chunk, then pick one at random.
		nodesHavingChunk := []string{}
		for j := range results {
			utils.DebugPrintln(results[j].Origin, results[j].FileName, results[j].Origin)
			for x := range results[j].ChunkMap {
				if results[j].ChunkMap[x] == uint64(i) {
					nodesHavingChunk = append(nodesHavingChunk, results[j].Origin)
				}
			}
		}
		chosenNode := nodesHavingChunk[rand.Intn(len(nodesHavingChunk))]
		order = append(order, chosenNode)
	}
	return order, nil
}

func (fileMatches *FileMatches) isFullyMatched(metafileHash []byte) bool {
	fileMatches.Mutex.RLock()
	defer fileMatches.Mutex.RUnlock()
	file, ok := fileMatches.Map[string(metafileHash)]
	if !ok {
		utils.DebugPrintln("Tried to check match for nonexistent file", string(metafileHash))
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
	utils.DebugPrintln("LENGTH THING: ", len(foundChunks), int(file[0].ChunkCount))
	if len(foundChunks) == int(file[0].ChunkCount) { // TODO FILESIZE
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
	peerster.FileSearchSessions.AddSession(keywords, budget)
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
	utils.DebugPrintln(reply.Results)
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
		utils.DebugPrintln(i, "I")
		file := peerster.SharedFiles.Map[i]
		for j := range keywords {
			utils.DebugPrintln(j, "J")
			utils.DebugPrintln(file.FileName, keywords[j], strings.Contains(file.FileName, keywords[j]))
			if strings.Contains(file.FileName, keywords[j]) { // TODO need to use REGEXP here? or what?
				foundFiles = append(foundFiles, file)
				break
			}
		}
	}
	utils.DebugPrintln(foundFiles, "FOUNDFILES")
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
		utils.DebugPrintln(i)
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
	// If we receive enough matches (MATCH_THRESHOLD) then we stop searching...?
	// Only if we're doing the doubling stuff, yaeh?
	if reply == nil {
		return
	}
	peerster.FileSearchSessions.FindMatchingSessions(*reply)
	peerster.FileMatches.addResults(*reply)

	// Now, we need ot know if it was a specified budget or not. If it was, we stop.
	// Otherwise, we check if we have enough matches (2) or if our budget is too high (32). Then we stop.
	// Otherwise, we need to search again with double budget.
}
