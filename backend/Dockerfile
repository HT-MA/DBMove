FROM golang:1.24-alpine AS build

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dbmove-api ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata docker-cli

WORKDIR /app
COPY --from=build /out/dbmove-api /app/dbmove-api

EXPOSE 8080
ENTRYPOINT ["/app/dbmove-api"]
