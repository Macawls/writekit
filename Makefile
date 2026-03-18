BINARY := writekit

.PHONY: build run dev clean

build:
	go build -o $(BINARY) ./cmd/writekit

run: build
	./$(BINARY)

dev:
	DEV=true go run ./cmd/writekit

clean:
	rm -f $(BINARY)
