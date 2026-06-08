FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /apidep .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates openssh-client

COPY --from=builder /apidep /apidep

ENTRYPOINT ["/apidep"]
