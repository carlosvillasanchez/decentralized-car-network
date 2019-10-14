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
    _httpPost(url + "/message", messageInput);
};

registerNode = () => {
    let nodeInput = _getValueFromElementById("node-input");
    _httpPost(url + "/node", nodeInput);
};

_listMessages = (newMessages) => {
    for (let i in newMessages) {
        let message = newMessages[i];
        console.log(message);
        let list = document.getElementById("chat-list");
        let li = document.createElement("li");
        let b = document.createElement("b");
        b.innerHTML = message["Origin"] + ":";
        let span = document.createElement("span");
        span.innerHTML = message["Text"];
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
}

let retrievedMessages = [];
_pollMessages = () => {
    let _addMessages = (response) => {
        let newMessages = [];
        for (let node in response) {
            for (let msg in response[node]) {
                let message = response[node][msg];
                let found = false;
                for (let i in retrievedMessages) {
                    if (retrievedMessages[i].Origin === message.Origin && retrievedMessages[i].ID === message.ID) {
                        found = true;
                    }
                }
                if (!found) {
                    newMessages.push(message);
                }
            }
        }
        retrievedMessages = retrievedMessages.concat(newMessages);
        _listMessages(newMessages)
    };
    setInterval(() => {
        _httpGet(url + "/message", [], _addMessages);
    }, 1000);
    setInterval(() => {
        _httpGet(url + "/node", [], _listPeers);
    }, 1000);
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