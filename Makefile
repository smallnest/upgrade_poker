.PHONY: build run test clean

BINARY=upgrade

build:
	go build -o $(BINARY) ./cmd/upgrade_poker/

run: build
	./$(BINARY)

test:
	go test -v -timeout 30s ./...

stress:
	go test -v -run TestAIPlayNoCrash -count=50 -timeout 120s ./...

clean:
	rm -f $(BINARY)
