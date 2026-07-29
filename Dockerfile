FROM golang:1.24.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/archivist-server ./cmd/archivist-server && \
    CGO_ENABLED=1 go build -o /out/archivist ./cmd/archivist && \
    CGO_ENABLED=1 go build -o /out/archivist-maintenance ./cmd/archivist-maintenance

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/archivist-server /usr/local/bin/archivist-server
COPY --from=build /out/archivist /usr/local/bin/archivist
COPY --from=build /out/archivist-maintenance /usr/local/bin/archivist-maintenance
COPY web ./web
RUN mkdir -p /data/uploads /data/storage
EXPOSE 8080
CMD ["archivist-server"]
