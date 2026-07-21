all:
	go build -ldflags="-s -w" -o docgo .

install:
	cp docgo /usr/local/bin/
