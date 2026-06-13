build:
	docker build --tag sknr/go-ws-reachability:latest .

run:
	docker run --rm sknr/go-ws-reachability

test:
	go test ./... -v

lint:
	go fmt ./...
	go vet ./...
	golangci-lint run

vuln:
	govulncheck ./...

verify: test lint vuln

build-manifest:
	# Build and push amd64
	docker build --platform linux/amd64 -t sknr/go-ws-reachability:latest-amd64 .
	docker push sknr/go-ws-reachability:latest-amd64
	
	# Build and push arm64
	docker build --platform linux/arm64 -t sknr/go-ws-reachability:latest-arm64 .
	docker push sknr/go-ws-reachability:latest-arm64
	
	# Combine into one tag
	docker manifest create --amend sknr/go-ws-reachability:latest \
		sknr/go-ws-reachability:latest-amd64 \
		sknr/go-ws-reachability:latest-arm64
	docker manifest push sknr/go-ws-reachability:latest
