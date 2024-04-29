DEPENDENCIES = flask websockets asyncio

all: https run

run:
	@python3 ./audio_server.py

https: ./key.pem ./cert.pem 
./cert.pem:
	openssl req -new -x509 -key key.pem -out cert.pem -days 365

./key.pem:
	openssl genrsa -out key.pem 2048

setup:
	@python3 -m pip install $(DEPENDENCIES) 
