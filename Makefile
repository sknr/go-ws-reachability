build:
	ENV GOOS=linux GOARCH=amd64 go build -o docker/ws-reachability main.go
	docker build --tag sknr/go-ws-reachability:latest .

run:
	docker run --rm sknr/go-ws-reachability
