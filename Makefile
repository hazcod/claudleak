all: run

run:
	go run ./cmd/claudleak/...

build:
	go build -o claudleak ./cmd/claudleak/... -verified-only
