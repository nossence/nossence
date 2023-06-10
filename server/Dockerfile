FROM golang:1.20-alpine AS builder

WORKDIR /build

COPY go.mod .
COPY go.sum .

COPY . .

RUN go mod download
RUN go build main.go

WORKDIR /dist

RUN cp /build/main .

FROM alpine

COPY --from=builder /dist/main /app/main

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["./main"]