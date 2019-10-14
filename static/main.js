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

url = "http://127.0.0.1:3333";

sendMessage = () => {
    let messageInput = _getValueFromElementById("message-input");
    _httpPost(url + "/new-message", messageInput);
};

registerNode = () => {
    let nodeInput = _getValueFromElementById("node-input");
    _httpPost(url + "/register-node", nodeInput);
};

_listMessages = (newMessages) => {
    for (let i in newMessages) {
        message = newMessages[i];
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
        _httpGet(url + "/get-messages", [], _addMessages);
    }, 1000)
};

_pollMessages();