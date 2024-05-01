registerProcessor("processor", class extends AudioWorkletProcessor {
    socket = null;
    isRecording = false;

    process([input], [output], parameters) {
        if(this.socket === null) {
            return true;
        }

        if(this.isRecording) {
            this.socket.send(input)
        }
        return true;
    }

    constructor() {
        super();
        console.log("Processor is created");
    }
});
