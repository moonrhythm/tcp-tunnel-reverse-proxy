FROM golang:1.16.2

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0
RUN mkdir -p /workspace
WORKDIR /workspace
ADD go.mod ./
RUN go mod download
ADD . .
RUN go build -o .build/tcp-tunnel-reverse-proxy -ldflags "-w -s" .

FROM gcr.io/moonrhythm-containers/go-scratch

WORKDIR /app

COPY --from=0 /workspace/.build/* ./
ENTRYPOINT ["/app/tcp-tunnel-reverse-proxy"]
