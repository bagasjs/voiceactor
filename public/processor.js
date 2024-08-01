registerProcessor("simple-processor", class extends AudioWorkletProcessor {
    socket = null;
    isRecording = false;

    process([input], [output], parameters) {
        if(this.isRecording && this.port && this.port.postMessage) {
            console.log(input)
            this.port.postMessage(input)
        }
        return true;
    }

    static get parameterDescriptors() {
        return [];
    }

    constructor({ isRecording }) {
        super();
        console.log("Processor is created");

        this.port.onmessage = ev => {
            console.log("Changing the recording state into", ev.data.isRecording);
            this.isRecording = ev.data.isRecording;
        }

        this.isRecording = isRecording;
    }
});
