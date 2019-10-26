package gossiper

import (
	"crypto"
	"fmt"
	"io/ioutil"
)

const SharedFilesPath = "./_SharedFiles/"
const ChunkSize = 8192

// Reads a file from the shared file path
func readSharedFile(fileName string) ([]byte, error) {
	return ioutil.ReadFile(SharedFilesPath + fileName)
}

// Divides a file into chunks of size ChunkSize (last chunk will have a dynamic size)
func chunkFile(file []byte) (chunks [][]byte) {
	for i := 0; i <= len(file)/ChunkSize; i++ {
		upperBound := (i + 1) * ChunkSize
		if (i+1)*ChunkSize > len(file) {
			upperBound = len(file)
		}
		chunk := file[i*ChunkSize : upperBound]
		chunks = append(chunks, chunk)
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
func reconstructAndSaveFile(chunks [][]byte) error {
	return nil
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
func (peerster *Peerster) indexReadFile(file []byte, fileName string) {
	chunks := chunkFile(file)
	metafile, metafileHash := computeMetafile(chunks)
	sharedFile := SharedFile{
		Metafile:     metafile,
		MetafileHash: metafileHash,
		FileSize:     0,
		FileName:     fileName,
	}
	peerster.SharedFiles[string(metafileHash)] = sharedFile
	peerster.FileChunks[string(metafileHash)] = chunks
}