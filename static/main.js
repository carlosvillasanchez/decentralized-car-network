let retrievedMessages = [];
let privateMessages = [];
let privateMessageTarget = "";

_getValueFromElementById = (id) => {
    return document.getElementById(id).value;
};

_httpPost = (url, body) => {
    let req = new XMLHttpRequest();
    req.open("POST", url, true);
    req.send(body)
};

_httpGet = (url, body, callback) => {
    let req = new XMLHttpRequest();
    req.open("GET", url, true);
    req.onreadystatechange = function() {
        if (req.readyState === 4 && req.status === 200)
            callback(JSON.parse(req.responseText));
    };
    req.send(body)
};

url = "http://localhost:8080";

sendMessage = () => {
    let messageInput = _getValueFromElementById("message-input");
    _httpPost(url + "/message", JSON.stringify({message: messageInput, destination: privateMessageTarget}));
};

registerNode = () => {
    let nodeInput = _getValueFromElementById("node-input");
    _httpPost(url + "/node", nodeInput);
};

selectPrivateMsgRecipient = (id) => {
    privateMessageTarget = id;
    document.getElementById("selected-recipient").innerHTML = "Selected recipient: " + id
};

_listMessages = (newMessages) => {
    for (let i in newMessages) {
        let message = newMessages[i];
        if (message["Text"] === ""){
            continue
        }
        let list = document.getElementById("chat-list");
        let li = document.createElement("li");
        let b = document.createElement("b");
        b.innerHTML = message["Origin"] + ":";
        let span = document.createElement("span");
        let privateString = "";
        if (message["ID"] === 0) {
            privateString = "PRIVATE: "
        }
        span.innerHTML = privateString + message["Text"];
        li.appendChild(b);
        li.appendChild(span);
        list.appendChild(li)
    }
};

_listPeers = (peers) => {
    let elem = document.getElementById("peer-list");
    elem.innerHTML = "";
    for (let i in peers) {
        let peer = peers[i];
        let li = document.createElement("li");
        li.innerHTML = peer;
        elem.appendChild(li);
    }
};

_listHopTable = (hops) => {
    let elem = document.getElementById("hop-list");
    elem.innerHTML = "";
    for (let i in hops) {
        let li = document.createElement("li");
        li.onclick = () => {
            selectPrivateMsgRecipient(i);
        };
        li.innerHTML = i;
        elem.appendChild(li);
    }
};

_pollMessages = () => {
    let _addMessages = (response) => {
        let newMessages = [];
        for (let type in response) {
            for (let node in response[type]) {
                for (let msg in response[type][node]) {
                    let message = response[type][node][msg];
                    let found = false;
                    for (let i in retrievedMessages) {
                        if (retrievedMessages[i].Origin === message.Origin && (message.ID !== 0 && retrievedMessages[i].ID === message.ID)) {
                            found = true;
                        }
                    }
                    if (!found) {
                        newMessages.push(message);
                    }
                }
            }
        }

        retrievedMessages = retrievedMessages.concat(newMessages);
        _listMessages(newMessages)
    };
    setInterval(() => {
        _httpGet(url + "/message", [], _addMessages);
        _httpGet(url + "/hop-table", [], _listHopTable);
        _httpGet(url + "/node", [], _listPeers);
    }, 1000)

};

_setId = (response) => {
    console.log(response);
    document.getElementById("title").innerHTML = "Peerster ID: " + response;
};
_getId = () => {
    _httpGet(url + "/id", [], _setId);
};

_getId();
_pollMessages();