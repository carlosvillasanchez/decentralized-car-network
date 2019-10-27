package gossiper

import (
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"net"
	"sync"
	"time"
)

type FileBeingDownloaded struct {
	MetafileHash   []byte
	Metafile       []byte
	Channel        chan messaging.DataReply
	CurrentChunk   int
	DownloadedData []byte
}

func (f *FileBeingDownloaded) getHashToSend() []byte {
	if f.Metafile == nil {
		return f.MetafileHash
	}
	upperBound := (f.CurrentChunk + 1) * ChunkSize
	if (f.CurrentChunk+1)*ChunkSize > len(f.Metafile) {
		upperBound = len(f.Metafile)
	}
	return f.Metafile[f.CurrentChunk*ChunkSize : upperBound]
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

func (d *DownloadingFiles) incrementCurrentChunk(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	f := d.Map[index]
	f.CurrentChunk++
	d.Map[index] = f
}

func (d *DownloadingFiles) getValue(index string) (*FileBeingDownloaded, bool) {
	d.Mutex.RLock()
	defer d.Mutex.RUnlock()
	file, ok := d.Map[index]
	return &file, ok
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
func (peerster *Peerster) downloadData(peerIdentifier string, hash []byte) {
	index := string(hash)
	_, ok := peerster.DownloadingFiles.getValue(index)
	if !ok {
		peerster.DownloadingFiles.setValue(index, FileBeingDownloaded{
			MetafileHash: hash,
			Channel:      make(chan messaging.DataReply),
			CurrentChunk: 0,
			Metafile:     nil,
		})
	}
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
				fileBeingDownloaded.DownloadedData = append(fileBeingDownloaded.DownloadedData, reply.Data...)
			}
			peerster.DownloadingFiles.setValue(index, *fileBeingDownloaded)
			peerster.DownloadingFiles.incrementCurrentChunk(index)
		case <-time.After(5 * time.Second):
		}
		value, ok = peerster.DownloadingFiles.getValue(index)
		if !ok {
			fmt.Printf("Warning: was unable to find the file download session when it should exist. Probably a bug")
		}
		peerster.downloadData(peerIdentifier, value.getHashToSend())
	}()
}

func (peerster *Peerster) startFileUpload(metafileHash []byte) {

}

func (peerster *Peerster) handleIncomingDataReply(reply *messaging.DataReply, originAddr net.UDPAddr) {
	// TODO We need to check if the thing doesnt have a metafile, if it doesnt then it was a metafile request
	index := string(reply.HashValue)
	fileBeingDownloaded, ok := peerster.DownloadingFiles.getValue(index)
	if !ok {
		// We aren't downloading this file, so why did we receive it? Who knows..
		// TODO is that a possible case? ??? ??? ????
		return
	}
	if fileBeingDownloaded.Metafile == nil {
		fileBeingDownloaded.Metafile = reply.Data
	} else {
		fileBeingDownloaded.DownloadedData = append(fileBeingDownloaded.DownloadedData, reply.Data...)
	}
	// We send a message through the session's channel to trigger the session deleting itself and starting a new one
	peerster.DownloadingFiles.confirmReceivedDataReply(index, *reply)
}

func (peerster *Peerster) handleIncomingDataRequest(request *messaging.DataRequest, originAddr net.UDPAddr) {
	if request.Destination == peerster.Name {
		//TODO we see if we have the file, and then send it back
		var data []byte
		file, ok := peerster.SharedFiles[string(request.HashValue)]
		if !ok {
			//TODO Its not a file, so must be a chunk
			chunk, _ := peerster.FileChunks[string(request.HashValue)]
			// Chunk can be nil here - which means we don't have the chunk, so sending back a nil value is correct
			data = chunk
		} else {
			// This means the request was for a metafile we have
			data = file.Metafile
		}
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
