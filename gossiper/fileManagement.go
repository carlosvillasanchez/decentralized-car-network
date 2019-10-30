package gossiper

import (
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
)

const SharedFilesPath = "./_SharedFiles/"
const DownloadedFilesPath = "./_Downloads/"
const ChunkSize = 8192

// Reads a file from the shared file path
func readSharedFile(fileName string) ([]byte, error) {
	return ioutil.ReadFile(SharedFilesPath + fileName)
}

// Divides a file into chunks of size ChunkSize (last chunk will have a dynamic size)
func chunkFile(file []byte) (chunks [][]byte, chunkHashes [][]byte) {
	for i := 0; i <= len(file)/ChunkSize; i++ {
		upperBound := (i + 1) * ChunkSize
		if (i+1)*ChunkSize > len(file) {
			upperBound = len(file)
		}
		chunk := file[i*ChunkSize : upperBound]
		chunks = append(chunks, chunk)
		sha256 := crypto.SHA256.New()
		sha256.Write(chunks[i])
		chunkHashes = append(chunkHashes, sha256.Sum(nil))
	}
	return
}

// Creates a metafile and a hash of the metafile from an array of file chunks
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

// Will reconstruct a file from a set of chunks, and save it in _Downloads
func reconstructAndSaveFile(downloaded FileBeingDownloaded) error {
	data := append([]byte{}, downloaded.DownloadedData...)
	return ioutil.WriteFile(DownloadedFilesPath+downloaded.FileName, data, 0644)
}

// Verifies that a file has a specific hash
func verifyFileChunk(requestedHash, retrievedData []byte) bool {
	sha256 := sha256.New()
	sha256.Write(retrievedData)
	retrievedDataHash := sha256.Sum(nil)
	for i := range requestedHash {
		if retrievedDataHash[i] != requestedHash[i] {
			return false
		}
	}
	return true
}

// A struct that contains all the metadata for a specific file
type SharedFile struct {
	Metafile     []byte
	MetafileHash []byte
	FileSize     int
	FileName     string
}

// Reads a file from the _SharedFiles folder and calls indexReadFile on it
func (peerster *Peerster) shareFile(fileName string) {
	file, err := readSharedFile(fileName)
	if err != nil {
		fmt.Printf("Could not read shared file, reason: %s \n", err)
		return
	}
	peerster.indexReadFile(file, fileName)
}

// Adds a read file to the peerster's internal data structure (chunks it and computes metafile/hash as well)
func (peerster *Peerster) indexReadFile(file []byte, fileName string) { //TODO remove fileName arg
	chunks, chunkHashes := chunkFile(file)
	metafile, metafileHash := computeMetafile(chunks)
	sharedFile := SharedFile{
		Metafile:     metafile,
		MetafileHash: metafileHash,
		FileSize:     0, // TODO filesize unnecessary? why did i add this
	}
	fmt.Printf("MetafileHash: %s, ChunkLength: %v \n", hex.EncodeToString(metafileHash), len(chunks)) // RemoveTag
	peerster.SharedFiles.Mutex.Lock()
	defer peerster.SharedFiles.Mutex.Unlock()
	peerster.SharedFiles.Map[string(metafileHash)] = sharedFile
	for i := range chunkHashes {
		fmt.Println(i, chunkHashes[i])
		peerster.FileChunks.Map[string(chunkHashes[i])] = chunks[i]
	}
}

func (peerster *Peerster) indexReconstructedFile(file FileBeingDownloaded) {
	sharedFile := SharedFile{
		Metafile:     file.Metafile,
		MetafileHash: file.MetafileHash,
		FileSize:     0,
	}
	fmt.Printf("Index reconstructed %v, %s", file.MetafileHash, file.Metafile)
	peerster.SharedFiles.Mutex.Lock()
	defer peerster.SharedFiles.Mutex.Unlock()
	peerster.SharedFiles.Map[string(file.MetafileHash)] = sharedFile // TODO needs to be mutex
	// if this is not mutex when youre reviewing i will buy you a beer
}
