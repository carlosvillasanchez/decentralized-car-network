package gossiper

import (
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"net"
	"sync"
	"time"
)

const HashSize = 32

type FileBeingDownloaded struct {
	MetafileHash   []byte
	Metafile       []byte
	Channel        chan messaging.DataReply
	CurrentChunk   int
	DownloadedData []byte
	FileName       string
}

func (f *FileBeingDownloaded) getHashToSend() []byte {
	if f.Metafile == nil {
		return f.MetafileHash
	}
	upperBound := (f.CurrentChunk + 1) * HashSize
	if (f.CurrentChunk+1)*HashSize > len(f.Metafile) {
		upperBound = len(f.Metafile)
	}
	lowerBound := f.CurrentChunk * HashSize
	if lowerBound > upperBound {
		lowerBound = 0
	}
	fmt.Println((f.CurrentChunk + 1) * HashSize)                                                                                                                  // RemoveTag
	fmt.Printf("THE HASH TO SEND IS: %v lowerb: %v higherb: %v, len: %v, \n", string(f.Metafile[lowerBound:upperBound]), lowerBound, upperBound, len(f.Metafile)) // RemoveTag
	return f.Metafile[lowerBound:upperBound]
}

type DownloadingFiles struct {
	Map   map[string]FileBeingDownloaded
	Mutex sync.RWMutex
}

/* TODO check if necessary
func (d *DownloadingFiles) decrementTimer(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	f := d.Map[index]
	f.Timer--
	d.Map[index] = f
} */

func (d *DownloadingFiles) confirmReceivedDataReply(index string, reply messaging.DataReply) {
	d.Mutex.RLock()
	defer d.Mutex.RUnlock()
	d.Map[index].Channel <- reply
}

func (d *DownloadingFiles) isFullyDownloaded(index string) bool {
	d.Mutex.RLock()
	defer d.Mutex.RUnlock()
	f := d.Map[index]
	return f.CurrentChunk*HashSize >= len(f.Metafile)
}

func (d *DownloadingFiles) incrementCurrentChunk(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	f := d.Map[index]
	f.CurrentChunk++
	d.Map[index] = f
}

func (d *DownloadingFiles) getValue(index string) (FileBeingDownloaded, bool) {
	d.Mutex.RLock()
	defer d.Mutex.RUnlock()
	file, ok := d.Map[index]
	return file, ok
}

func (d *DownloadingFiles) setValue(index string, value FileBeingDownloaded) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	d.Map[index] = value
}

func (d *DownloadingFiles) deleteValue(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	delete(d.Map, index)
}

func (peerster *Peerster) sendDataRequest(destination string, hash []byte) {
	request := messaging.DataRequest{
		Origin:      peerster.Name,
		Destination: destination,
		HopLimit:    11,
		HashValue:   hash,
	}
	// TODO start a "download session", marked by the metafile hash + destination?
	peerster.handleIncomingDataRequest(&request, messaging.StringAddrToUDPAddr(peerster.GossipAddress))
}

// Starts a download of a specific chunk (or a metafile)
func (peerster *Peerster) downloadData(peerIdentifier string, previousDownloadSession FileBeingDownloaded) {
	hash := previousDownloadSession.getHashToSend()
	index := string(hash)
	peerster.DownloadingFiles.setValue(index, previousDownloadSession)
	peerster.sendDataRequest(peerIdentifier, hash)
	go func() {
		value, ok := peerster.DownloadingFiles.getValue(index)
		if !ok {
			return
		}
		select {
		case reply := <-value.Channel:
			fileBeingDownloaded, ok := peerster.DownloadingFiles.getValue(index)
			if !ok {
				// We aren't downloading this file, so why did we receive it? Who knows..
				// TODO is that a possible case? ??? ??? ????
				return
			}
			if fileBeingDownloaded.Metafile == nil {
				fileBeingDownloaded.Metafile = reply.Data
			} else {
				// TODO verify that data is correct
				fileBeingDownloaded.DownloadedData = append(fileBeingDownloaded.DownloadedData, reply.Data...)
				fileBeingDownloaded.CurrentChunk++
				peerster.FileChunks[string(reply.HashValue)] = reply.Data //TODO make mutex!!!!! THREAD SAFETY!!!!
				// if youre reviewing this and this isnt thread safe i will buy you a beer
			}
			peerster.DownloadingFiles.setValue(index, fileBeingDownloaded)

		case <-time.After(5 * time.Second):
			fmt.Printf("DownloadingFiles session timeout with hash %v \n", hash)
		}
		value, ok = peerster.DownloadingFiles.getValue(index)
		if !ok {
			fmt.Printf("Warning: was unable to find the file download session when it should exist. Probably a bug")
		}
		if peerster.DownloadingFiles.isFullyDownloaded(index) {
			err := reconstructAndSaveFile(value)
			if err != nil {
				fmt.Printf("Warning: Could not reconstruct/save file, reason: %s \n", err)
			} else {
				peerster.indexReconstructedFile(value)
			}
			peerster.DownloadingFiles.deleteValue(index)
		} else {
			// At this point, we either request the same hash over again (because of timeout)
			// or we request the next hash (because we incremented currentChunk,
			// getHashToSend will return the next hash)
			peerster.downloadData(peerIdentifier, value)
		}
	}()
}

func (peerster *Peerster) startFileUpload(metafileHash []byte) {

}

func (peerster *Peerster) handleIncomingDataReply(reply *messaging.DataReply, originAddr net.UDPAddr) {
	if reply == nil {
		return
	}
	fmt.Println("Reply for file has come. Values:", reply.Data, originAddr.String(), reply.Origin) // RemoveTag
	// TODO We need to check if the thing doesnt have a metafile, if it doesnt then it was a metafile request
	if reply.Destination == peerster.Name {
		index := string(reply.HashValue)
		// We send a message through the session's channel to trigger starting a new one with the next request
		// or if its finished, reconstruct/save the file
		peerster.DownloadingFiles.confirmReceivedDataReply(index, *reply)
	} else if reply.HopLimit == 0 {
		return
	} else {
		reply.HopLimit--
		peerster.nextHopRoute(&messaging.GossipPacket{DataReply: reply}, reply.Destination)
	}
}

func (peerster *Peerster) handleIncomingDataRequest(request *messaging.DataRequest, originAddr net.UDPAddr) {
	if request == nil {
		return
	}
	fmt.Println("Request for file has come. Values:", request.Destination, request.HashValue, originAddr.String(), request.Origin) // RemoveTag

	if request.Destination == peerster.Name {
		//TODO we see if we have the file, and then send it back
		var data []byte
		file, ok := peerster.SharedFiles[string(request.HashValue)]
		if !ok {
			//TODO Its not a file, so must be a chunk
			chunk, ok := peerster.FileChunks[string(request.HashValue)]
			if !ok {
				fmt.Println("Warning: A file request requested a chunk we don't have.", request.HashValue)
			}
			fmt.Println("He wants chunk.") // RemoveTag
			// Chunk can be nil here - which means we don't have the chunk, so sending back a nil value is correct
			data = chunk
		} else {
			// This means the request was for a metafile we have
			fmt.Println("He wants metafile.", file.Metafile) // RemoveTag
			data = file.Metafile
		}
		fmt.Println("The data is: ", data) // RemoveTag
		reply := &messaging.DataReply{
			Origin:      peerster.Name,
			Destination: request.Origin,
			HopLimit:    11, //TODO make constant
			HashValue:   request.HashValue,
			Data:        data,
		}
		// We send the DataReply into our incoming data reply handler, sending it into the network as normal
		peerster.handleIncomingDataReply(reply, messaging.StringAddrToUDPAddr(peerster.GossipAddress))
	} else if request.HopLimit == 0 {
		return
	} else {
		request.HopLimit--
		peerster.nextHopRoute(&messaging.GossipPacket{DataRequest: request}, request.Destination)
	}
}
