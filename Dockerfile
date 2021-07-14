#FROM scratch // does not work without root certificates
FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
WORKDIR /app
COPY docker/ .
VOLUME /app/data
CMD ["/app/ws-reachability"]
