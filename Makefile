BINARY      = ias_sti
IMAGE       = ias_sti:latest

.PHONY: build build-linux build-mac docker-build docker-run clean

build: build-linux

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY) .

build-mac:
	CGO_ENABLED=0 GOOS=darwin go build -o $(BINARY) .

docker-build: build-linux
	docker build -t $(IMAGE) .

docker-run:
	docker run -p 8080:8080 --env-file .env $(IMAGE)

clean:
	rm -f $(BINARY)
