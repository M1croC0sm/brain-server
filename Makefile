.PHONY: build run test clean deps dev install

# Build the server binary
build:
	go build -o brain-server ./cmd/brain-server

# Run the server (requires env vars or .env file)
run: build
	./brain-server

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f brain-server
	rm -f brain.db
	rm -f test-brain.db
	rm -rf test-vault

# Development run with sample config
dev: build
	mkdir -p ./test-vault/{Ideas,Projects,Financial/Ledger,Health,Life,Log,Letters/Daily,Letters/Weekly,Research/Ideas}
	BRAIN_PORT=8080 \
	BRAIN_VAULT_PATH=./test-vault \
	BRAIN_DB_PATH=./brain.db \
	BRAIN_TOKEN_WOLF=dev_token_wolf \
	./brain-server

# Install systemd service (run as root)
install: build
	chmod +x deploy/install.sh
	sudo deploy/install.sh

# View logs
logs:
	journalctl -u brain-server -f

# Restart service
restart:
	sudo systemctl restart brain-server

# Status
status:
	sudo systemctl status brain-server
