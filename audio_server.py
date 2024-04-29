import os
import threading
from flask import Flask, json, redirect, render_template
import asyncio
import websockets
import ssl
from datetime import datetime 

HOSTNAME="0.0.0.0"
HTTP_PORT=8000
WS_PORT=8001

app = Flask(__name__, 
            template_folder="views", 
            static_folder="public",
            static_url_path="/public"
            )

@app.route("/capture")
def capture():
    # TODO: start the websocket server that will receive the audio input
    # TODO: send the html UI
    return render_template("capture.html")

@app.route("/", methods=["GET"])
def index():
    return redirect("/capture")

def handle_audio_stream_data(data):
    print(data)
    pass

async def handle_websocket(ws, path):
    client_token = None
    async for data in ws:
        message = json.loads(data)
        match message["type"]:
            case "PING":
                response = { "type" : "PONG", "data" : "PONG" }
                await ws.send(json.dumps(response))
            case "AUDIOSTREAMINGSERVICE_SEND":
                if client_token != None:
                    if "token" in message and message["token"] == client_token:
                        handle_audio_stream_data(data);
                        response = { "type" : "AUDIOSTREAMINGSERVICE_RECEIVED", "data": "Data streaming success." }
                        await ws.send(json.dumps(response))
                    else:
                        response = { "type" : "ERROR", "data": "Invalid token could not receive the audio stream data." }
                        await ws.send(json.dumps(response))
                else:
                    response = { "type" : "ERROR", "data": "Server is currently not locked. This should be unreachable." }
                    await ws.send(json.dumps(response))

            case "AUDIOSTREAMINGSERVICE_UNLOCK":
                if client_token != None:
                    if "token" in message and message["token"] == client_token:
                        response = { "type" : "AUDIOSTREAMINGSERVICE_UNLOCKED", "data": "Successfully unlocking the server." }
                        await ws.send(json.dumps(response))
                    else:
                        response = { "type" : "ERROR", "data": "Invalid token could not unlock the audio streaming service." }
                        await ws.send(json.dumps(response))
                else:
                    response = { "type" : "ERROR", "data": "Server is currently not locked. This should be unreachable." }
                    await ws.send(json.dumps(response))

            case "AUDIOSTREAMINGSERVICE_LOCK":
                if client_token != None:
                    response = { "type" : "ERROR", "data" : "Another client has already locked the server." }
                    await ws.send(json.dumps(response))
                else:
                    timestamp = datetime.now().timestamp()
                    client_token = f"client-token-{timestamp}"
                    response = { "type" : "AUDIOSTREAMINGSERVICE_LOCKED", "data": client_token }
                    await ws.send(json.dumps(response))
            case _:
                print(message)
                response = { "type" : "ERROR", "data" : "Unknown message type." }
                await ws.send(json.dumps(response))

async def start_websocket_server():
    ssl_context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    certfile = os.path.join(os.getcwd(), "cert.pem")
    keyfile = os.path.join(os.getcwd(), "key.pem")
    ssl_context.load_cert_chain(certfile=certfile, keyfile=keyfile)
    async with websockets.serve(handle_websocket, "0.0.0.0", WS_PORT, ssl=ssl_context):
        print(f"Websocket server is started on ws(s)://0.0.0.0:{WS_PORT}")
        await asyncio.Future()

def run_websocket_app():
    asyncio.run(start_websocket_server())

def run_flask_app():
    app.run(host=HOSTNAME, port=HTTP_PORT, ssl_context=('cert.pem', 'key.pem'))

if __name__ == "__main__":
    print("Starting audio server do CTRL-c twice to close the server")
    websocket_app_thread = threading.Thread(target=run_websocket_app)
    flask_app_thread = threading.Thread(target=run_flask_app)
    try:
        websocket_app_thread.start()
        flask_app_thread.start()
        flask_app_thread.join()
        websocket_app_thread.join()
    except KeyboardInterrupt:
        print("Closing the application")

