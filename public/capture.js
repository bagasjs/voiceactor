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

(async () => {
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
        addNewLog(LOG_INFO, "getUserMedia is supported");
    } else {
        addNewLog(LOG_ERROR, "getUserMedia is not supported");
        return;
    }

    const WShostname = location.hostname;
    const WSProtocol = location.protocol === "https:" ? "wss" : "ws";
    let wsURL = `${WSProtocol}://${WShostname}:8001`
    let wsconn = null;
    try {
        wsconn = new WebSocket(wsURL);
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

    let processorNode = null;
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const audioContext = new AudioContext();
    await audioContext.audioWorklet.addModule("processor.js")
    processorNode = new AudioWorkletNode(audioContext, "simple-processor", {
        processorOptions: {
            isRecording: false,
        },
    });

    processorNode.onprocessorerror = ev => {
        console.error(ev);
    }

    processorNode.port.onmessage = ev => {
        // Send the data from processor worklet
        const data = ev.data;
        wsconn.send(new Blob(data));
    }

    const setProcessorRecordingState = (isRecording) => {
        if(processorNode && processorNode.port) {
            processorNode.port.postMessage({ isRecording });
        }
    }

    const source = audioContext.createMediaStreamSource(stream);
    source.connect(processorNode);
    processorNode.connect(audioContext.destination);

    wsconn.onmessage = ev => {
        const data = ev.data;
        const msg = JSON.parse(data);
        switch(msg.type) { 
            case "PONG":
                {
                    addNewLog(LOG_INFO, "Server is responding to our PONG")
                } break;
            case "AUDIOSTREAMINGSERVICE_LOCKED":
                {
                    captureButtonState = captureButtonState_Recording;
                    captureButton.innerText = "Stop";
                    audioStreamToken = msg.data
                    setProcessorRecordingState(true);

                    // recorder.start(100);
                    // recorder.ondataavailable = ({ data }) => {
                    //     wsconn.send(new Blob([data], {type: "audio/ogg; codes=opus"}))
                    // }
                    addNewLog(LOG_ERROR, "Audio streaming started");
                } break;
            case "AUDIOSTREAMINGSERVICE_UNLOCKED":
                {
                    // recorder.stop();
                    setProcessorRecordingState(false);
                    captureButton.innerText = "Record";
                    audioStreamToken = null;
                    captureButtonState = captureButtonState_Idle;
                    addNewLog(LOG_ERROR, "Audio streaming stoped");
                } break;
            case "ERROR":
                {
                    addNewLog(LOG_ERROR, msg.data);
                } break;
            case "AUDIOSTREAM_RECEIVED":
                {
                } break;
            default:
                {
                    addNewLog(LOG_ERROR, `Unknown server message ${msg.type}`);
                } break;
        }
    }


    // const recorder = new MediaRecorder(stream);
    captureButton.onclick = () => {
        if(wsconn.readyState == WebSocket.OPEN) {
            if(captureButtonState == captureButtonState_Idle) {
                if(audioStreamToken != null) {
                    addNewLog(LOG_FATAL, "Unreachable state where currently is not recording but there's audioStreamToken");
                }
                wsconn.send(JSON.stringify({ type: "AUDIOSTREAMINGSERVICE_LOCK" }))
            } else {
                if(audioStreamToken == null) {
                    addNewLog(LOG_FATAL, "Unreachable state where currently is recording but no audioStreamToken");
                }
                console.log("Trying to unlock the audio streaming")
                wsconn.send(JSON.stringify({ type: "AUDIOSTREAMINGSERVICE_UNLOCK", token: audioStreamToken }))
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
