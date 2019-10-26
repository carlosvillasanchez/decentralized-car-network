package gossiper

import (
	"crypto"
	"fmt"
	"io/ioutil"
)

const SharedFilesPath = "/_SharedFiles/"
const ChunkSize = 8192

func readSharedFile(fileName string) ([]byte, error) {
	return ioutil.ReadFile(SharedFilesPath + fileName)
}

func chunkFile(file []byte) (chunks [][]byte) {
	for i := 0; i < len(file)/ChunkSize; i++ {
		//TODO do not pad the last chunk to ChunkSize, it needs to be dynamically sized
		chunk := file[i*ChunkSize : (i+1)*ChunkSize]
		chunks = append(chunks, chunk)
	}
	return
}

func computeMetafile(chunks [][]byte) (metafile []byte, metafileHash []byte) {
	for i := range chunks {
		sha256 := crypto.SHA256.New()
		sha256.Write(chunks[i])
		metafile = append(metafile, sha256.Sum(nil)...)
	}
	sha256 := crypto.SHA256.New()
	sha256.Write(metafile)
	metafileHash = sha256.Sum(nil)
	return
}

type SharedFile struct {
	Metafile     []byte
	MetafileHash []byte
	FileSize     int
	FileName     string
}

func (peerster *Peerster) shareFile(fileName string) {
	file, err := readSharedFile(fileName)
	if err != nil {
		fmt.Printf("Could not read shared file, reason: %s \n", err)
		return
	}
	peerster.indexReadFile(file, fileName)
}

func (peerster *Peerster) indexReadFile(file []byte, fileName string) {
	metafile, metafileHash := computeMetafile(chunkFile(file))
	sharedFile := SharedFile{
		Metafile:     metafile,
		MetafileHash: metafileHash,
		FileSize:     0,
		FileName:     fileName,
	}
	peerster.SharedFiles = append(peerster.SharedFiles, sharedFile)
}
