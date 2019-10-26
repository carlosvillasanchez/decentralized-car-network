package gossiper

import (
	"fmt"
	"github.com/tormey97/Peerster/messaging"
	"net"
	"sync"
	"time"
)

type FileBeingDownloaded struct {
	MetafileHash []byte
	Metafile     []byte
	Timer        int
	CurrentChunk int
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

func (d *DownloadingFiles) decrementTimer(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	f := d.Map[index]
	f.Timer--
	d.Map[index] = f
}

func (d *DownloadingFiles) incrementCurrentChunk(index string) {
	d.Mutex.Lock()
	defer d.Mutex.Lock()
	f := d.Map[index]
	f.CurrentChunk++
	d.Map[index] = f
}

func (d *DownloadingFiles) getValue(index string) (file FileBeingDownloaded, ok bool) {
	d.Mutex.RLock()
	defer d.Mutex.RUnlock()
	file, ok = d.Map[index]
	return
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
	if ok {
		peerster.DownloadingFiles.deleteValue(index)
	}
	peerster.DownloadingFiles.setValue(index, FileBeingDownloaded{
		MetafileHash: hash,
		Timer:        5,
		CurrentChunk: 0,
		Metafile:     nil,
	})
	peerster.sendDataRequest(peerIdentifier, hash)
	go func() {
		for {
			value, ok := peerster.DownloadingFiles.getValue(index)
			if !ok {
				return
			}
			if value.Timer <= 0 {
				break
			}
			peerster.DownloadingFiles.decrementTimer(index)
			peerster.DownloadingFiles.Mutex.RLock() // This might cause problems
			time.Sleep(1 * time.Second)
			peerster.DownloadingFiles.Mutex.RUnlock()
		}
		value, ok := peerster.DownloadingFiles.getValue(index)
		if !ok {
			// In this case, the chunk was received, so we
			return
		}
		fmt.Printf("Timed out. We are now retransmitting the request \n")
		peerster.downloadData(peerIdentifier, hash)
	}()
}

func (peerster *Peerster) startFileUpload(metafileHash []byte) {

}

func (peerster *Peerster) handleIncomingDataReply(reply *messaging.DataReply, originAddr net.UDPAddr) {
	index := reply.Origin
	fileBeingDownloaded, ok := peerster.DownloadingFiles.getValue(reply)

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
