ARG GO_VERSION=1.16

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk update && apk add alpine-sdk git

RUN mkdir -p /bot
WORKDIR /bot

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o app main.go

FROM alpine:latest

RUN apk update && apk add ca-certificates

RUN mkdir -p /bot
WORKDIR /bot
COPY --from=builder /bot/app .

ENTRYPOINT ["./app"]