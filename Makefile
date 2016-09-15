.PHONY: all clean build install

BIN := $(GOPATH)/bin/gitch

all: install

clean:
	rm -f $(BIN)

build: clean
	go build

install: clean
	go install
