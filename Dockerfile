FROM golang:alpine as builder

ENV GOPROXY https://proxy.golang.org,direct

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk update --no-cache \
    && apk add --no-cache tzdata upx openssl \
    && update-ca-certificates

WORKDIR /app

COPY go.* ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o /bin/web3rpcproxy ./cmd/main.go
RUN upx -9 /bin/web3rpcproxy

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /bin/web3rpcproxy /bin/web3rpcproxy
COPY --from=builder --chown=nonroot /app/config /app/config

EXPOSE 8080
EXPOSE 8000

ENTRYPOINT ["/bin/web3rpcproxy"]