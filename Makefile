BINARY := booksmith
DIST   := dist

.PHONY: all build test vet fmt clean windows mac install

all: test build

build:
	go build -o $(DIST)/$(BINARY) .

windows:
	GOOS=windows GOARCH=amd64 go build -o $(DIST)/booksmith.exe .

mac:
	GOOS=darwin GOARCH=arm64 go build -o $(DIST)/booksmith-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o $(DIST)/booksmith-darwin-amd64 .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf $(DIST)
