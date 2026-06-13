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