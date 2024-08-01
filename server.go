package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/gorilla/websocket"
)

type TransferDataInfo struct {
	Token uint64
	Data  []byte
}

type Environment struct {
	TransferDataCh    chan TransferDataInfo
	LockAudioWorkerCh chan *WSClient
}

type Config struct {
	Secure           bool
	DebugEnvironment bool
	Hostname         string
	HttpPort         uint
	WsPort           uint
	StaticDirPath    string
	StaticUrlPath    string
	ResultFilePath   string
	CertFile         string
	KeyFile          string

	RemoteAddr string
}

type AudioWorker struct {
	Data  []byte
	Batch []byte

	locker    *WSClient
	lockToken uint64

	ResultFilePath string

    stream rl.AudioStream

	environ Environment
}

func NewAudioWorker(config Config, environ Environment) *AudioWorker {
	return &AudioWorker{
		Data:           nil,
		Batch:          make([]byte, 0),
		ResultFilePath: config.ResultFilePath,

		environ: environ,
	}
}

func (a *AudioWorker) SaveResultAndResetState() {
	f, err := os.Create(a.ResultFilePath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.Write(a.Batch)
	if err != nil {
		log.Fatal(err)
	}

	a.Batch = make([]byte, 0)
}

func bytesToFloat32(data []byte) []float32 {
    // Create a slice to store the float32 values
    floats := make([]float32, 0)

    for len(data) > 4{
        data = data[4:]

        bits := binary.LittleEndian.Uint32(data[:4])
        value := math.Float32frombits(bits)
        floats = append(floats, value)
    }

    return floats
}

func (a *AudioWorker) ReceiveAndCacheData(data []byte) {
    samples := bytesToFloat32(data)
    log.Println("Received data with size: ", len(data), " > float samples: ", len(samples))
    rl.UpdateAudioStream(a.stream, samples, 128)

	if a.Data != nil {
		a.Batch = append(a.Batch, a.Data...)
	}
	a.Data = make([]byte, len(data))
    copy(a.Data, data)

}

func (a *AudioWorker) StartWork() {
	for {
		select {
		case info := <-a.environ.TransferDataCh:
            // log.Printf("Received audio data %d", len(info.Data))
            // if info.Token == a.locker.Token {
            // }
			a.ReceiveAndCacheData(info.Data)
		case client := <-a.environ.LockAudioWorkerCh:
			if client != nil {
				a.locker = client
				a.lockToken = uint64(time.Now().Unix())
				a.locker.setTokenCh <- a.lockToken
			} else {
				a.SaveResultAndResetState()
				a.locker = nil
				a.lockToken = 0
			}
		}
	}
}

type WSClient struct {
	Conn       *websocket.Conn
	lockToken  uint64
	setTokenCh chan uint64
	environ    Environment
}

func NewWSClient(conn *websocket.Conn, environ Environment) *WSClient {
	return &WSClient{
		Conn:       conn,
		environ:    environ,
		lockToken:  0,
		setTokenCh: make(chan uint64),
	}
}

func (c *WSClient) SendSimpleResponse(messageType string, messageData string) error {
	response := map[string]string{"type": messageType, "data": messageData}
	return c.Conn.WriteJSON(response)
}

func (client *WSClient) HandleClientTextMessage(data []byte) error {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}
	messageType, ok := message["type"].(string)
	if !ok {
		return fmt.Errorf("Expecting client text message to has `type` field")
	}

	switch messageType {
	case "PING":
		return client.SendSimpleResponse("PONG", "PONG")

	case "AUDIOSTREAMINGSERVICE_LOCK":
		log.Println("[INFO] Client tried to lock the audio worker")
		client.environ.LockAudioWorkerCh <- client
		client.lockToken = <-client.setTokenCh
		if client.lockToken == 0 {
			return client.SendSimpleResponse("ERROR", "Failed to lock audio streaming service. there's another client lock it")
		}
		return client.SendSimpleResponse("AUDIOSTREAMINGSERVICE_LOCKED", "Audio streaming service is locked succesfully")

	case "AUDIOSTREAMINGSERVICE_UNLOCK":
		log.Println("[INFO] Client unlocking the audio worker")
		if client.lockToken == 0 {
			return client.SendSimpleResponse("ERROR", "You are not locking the audio streaming service")
		}
		client.environ.LockAudioWorkerCh <- nil
		client.lockToken = 0
		return client.SendSimpleResponse("AUDIOSTREAMINGSERVICE_UNLOCKED", "Audio streaming service is unlocked successfully")

	default:
		return client.SendSimpleResponse("ERROR", "Unknown message data")

	}
}

func (client *WSClient) HandleClientBinaryMessage(data []byte) error {
	info := TransferDataInfo{Data: data, Token: 0}
	client.environ.TransferDataCh <- info
	return nil
}

// func (client *WSClient) ChannelsHandler() {
// 	select {
//     case token := <-client.setTokenCh:
//         client.lockToken = token
// 	}
// }

func (client *WSClient) Serve() {
	defer client.Conn.Close()
	for {
		mt, message, err := client.Conn.ReadMessage()
		if err != nil {
			log.Print("Read error: ", err)
			continue
		}

		switch mt {
		case websocket.TextMessage:
			err = client.HandleClientTextMessage(message)
		case websocket.BinaryMessage:
			err = client.HandleClientBinaryMessage(message)
		default:
			err = fmt.Errorf("Unknown websocket message type")
		}

		if err != nil {
			fmt.Println(err)
		}
	}
}

func StartWebSocketServer(config Config, environ Environment) {
    protocol := "ws"
    if config.Secure {
        protocol = "wss"
    }

    host := fmt.Sprint(config.Hostname, ":", config.WsPort)
    informativeHostAddr := fmt.Sprint(protocol, "://", config.RemoteAddr, ":", config.WsPort)
	log.Println("HTTP Server is started at", host, "(", informativeHostAddr, ")")

	var upgrader = websocket.Upgrader{}
	if config.DebugEnvironment {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("Upgrade error: ", err)
			return
		}
		client := NewWSClient(c, environ)
		go client.Serve()
	})

	if config.Secure {
		if err := http.ListenAndServeTLS(host, config.CertFile, config.KeyFile, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := http.ListenAndServe(host, nil); err != nil {
			log.Fatal(err)
		}
	}
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, address := range addrs {
		// Check if the address is not a loopback and is an IPv4 address
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

func usage() {
    fmt.Fprint(os.Stderr, "Usage: server [secure=secure|<empty>]")
    os.Exit(2)
}

func main() {
    flag.Usage = usage
    flag.Parse()

    rl.InitAudioDevice()
    defer rl.CloseAudioDevice()

	config := Config{}
	config.Hostname = "0.0.0.0"
	config.WsPort = 8001
	config.HttpPort = 8000
	config.DebugEnvironment = true
	config.StaticDirPath = "./public"
	config.StaticUrlPath = "/"
	config.ResultFilePath = "result.dat"
	config.Secure = false 
	config.CertFile = "./cert.pem"
	config.KeyFile = "./key.pem"

    config.RemoteAddr = GetLocalIP()


    if arg := flag.Arg(0); strings.Compare(arg, "secure") == 0 {
        config.Secure = true
    }

	environ := Environment{}
	environ.TransferDataCh = make(chan TransferDataInfo)
	environ.LockAudioWorkerCh = make(chan *WSClient)

	audioWorker := NewAudioWorker(config, environ)
    rl.SetAudioStreamBufferSizeDefault(1024);
    audioWorker.stream = rl.LoadAudioStream(44100, 32, 2)
    defer rl.UnloadAudioStream(audioWorker.stream)
    rl.PlayAudioStream(audioWorker.stream)

	go StartHttpServer(config)
	go audioWorker.StartWork()
	StartWebSocketServer(config, environ)
}
