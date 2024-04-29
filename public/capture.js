const captureButton = document.getElementById("capture-btn");
const pingServerButton = document.getElementById("ping-server-btn")
const appLogs = document.getElementById("app-logs");

const LOG_INFO = "info";
const LOG_ERROR = "error";
const LOG_FATAL = "fatal";

const captureButtonState_Idle = 0;
const captureButtonState_Recording = 1;
let captureButtonState = captureButtonState_Idle;
let audioStreamToken = null;

const addNewLog = (type, message) =>  {
    const log = document.createElement("p");
    log.classList.add(type)
    log.appendChild(document.createTextNode(message));
    appLogs.appendChild(log);
}

let hostname = location.hostname;
let wsconn = null;

(async () => {
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
        addNewLog(LOG_INFO, "getUserMedia is supported");
    } else {
        addNewLog(LOG_ERROR, "getUserMedia is not supported");
        return;
    }

    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const recorder = new MediaRecorder(stream);

    try {
        const wsProtocol = location.protocol === "https:" ? "wss" : "ws";
        wsconn = new WebSocket(`${wsProtocol}://${hostname}:8001`);
    } catch(err) {
        addNewLog(LOG_FATAL, `Failed to connect into websocket: ${err}`);
        return;
    }
    wsconn.onopen = () => {
        addNewLog(LOG_INFO, "Websocket connection is opened");
    }

    wsconn.onclose = () => {
        addNewLog(LOG_INFO, "Websocket connection is closed");
    }

    wsconn.onmessage = ev => {
        const data = ev.data;
        const msg = JSON.parse(data);
        switch(msg.type) { 
            case "PONG":
                {
                    addNewLog(LOG_INFO, "Server is responding to our PONG")
                } break;
            case "AUDIOSTREAMINGSERVICE_RECEIVED":
                {
                    addNewLog(LOG_ERROR, "Debugging purpose only");
                } break;
            case "AUDIOSTREAMINGSERVICE_LOCKED":
                {
                    captureButtonState = captureButtonState_Recording;
                    captureButton.innerText = "Stop";
                    audioStreamToken = msg.data

                    recorder.start()
                    addNewLog(LOG_ERROR, "Audio streaming started");
                    recorder.ondataavailable = ({ data }) => {
                        // TODO: maybe process the data first 
                        wsconn.send(JSON.stringify({
                            type: "AUDIOSTREAMINGSERVICE_SEND",
                            data: data,
                            token: audioStreamToken,
                        }));
                    }
                } break;
            case "AUDIOSTREAMINGSERVICE_UNLOCKED":
                {
                    recorder.stop();
                    captureButton.innerText = "Record";
                    audioStreamToken = null;
                    addNewLog(LOG_ERROR, "Audio streaming stoped");
                } break;
            case "ERROR":
                {
                    addNewLog(LOG_ERROR, msg.data);
                } break;
            default:
                {
                    addNewLog(LOG_ERROR, `Unknown server message ${msg.type}`);
                } break;
        }
    }

    captureButton.onclick = () => {
        if(wsconn.readyState == WebSocket.OPEN) {
            if(captureButtonState == captureButtonState_Recording) {
                if(audioStreamToken == null) {
                    addNewLog(LOG_FATAL, "Unreachable state where currently is recording but no audioStreamToken");
                }
                wsconn.send(JSON.stringify({ type: "AUDIOSTREAMINGSERVICE_UNLOCK", token: audioStreamToken }))
            } else {
                if(audioStreamToken != null) {
                    addNewLog(LOG_FATAL, "Unreachable state where currently is not recording but there's audioStreamToken");
                }
                wsconn.send(JSON.stringify({ type: "AUDIOSTREAMINGSERVICE_LOCK" }))
            }
        }
    }

    pingServerButton.onclick = () => {
        if(wsconn.readyState == WebSocket.OPEN) {
            wsconn.send(JSON.stringify({ 
                type: "PING",
                data: "Hello!",
            }));
        }
    }
})();
