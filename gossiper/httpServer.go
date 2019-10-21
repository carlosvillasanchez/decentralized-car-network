package gossiper

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (peerster *Peerster) handleNewMessage(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		fmt.Printf("Could not read message from frontend, reason: %s \n", err)
	}
	peerster.clientReceive(buffer[:n])
}

func (peerster *Peerster) handleRegisterNode(w http.ResponseWriter, req *http.Request) {
	buffer := make([]byte, 1024)
	n, err := req.Body.Read(buffer)
	if err != nil {
		fmt.Printf("Could not register new node from frontend, reason: %s \n", err)
	}
	peerster.addToKnownPeers(string(buffer[:n]))
}

func sendValueAsJson(w http.ResponseWriter, req *http.Request, val interface{}) {
	b, err := json.Marshal(val)
	if err != nil {
		fmt.Printf("Could not encode received msgs as json, reason: %s \n", err)
		return
	}
	w.WriteHeader(200)
	_, err = w.Write(b)
	if err != nil {
		fmt.Printf("Could not send messages to frontend, reason: %s \n", err)
		return
	}
}

func (peerster *Peerster) handleMessage(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		sendValueAsJson(w, req, peerster.ReceivedMessages)
	case http.MethodPost:
		peerster.handleNewMessage(w, req)
	}
}

func (peerster *Peerster) handlePeers(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		sendValueAsJson(w, req, peerster.KnownPeers)
	case http.MethodPost:
		peerster.handleRegisterNode(w, req)
	}
}

func (peerster *Peerster) handleId(w http.ResponseWriter, req *http.Request) {
	sendValueAsJson(w, req, peerster.Name)
}

func (peerster *Peerster) ListenFrontend() {
	http.HandleFunc("/message", peerster.handleMessage)
	http.HandleFunc("/node", peerster.handlePeers)
	http.HandleFunc("/id", peerster.handleId)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Could not listen to the frontend, reason: %s \n", err)
	}
}
