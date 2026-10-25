FROM golang:1.26-alpine AS builder 

WORKDIR /app

COPY  go.mod go.sum* ./
RUN go mod download 

COPY . . 

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/agent main.go

FROM alpine:latest

WORKDIR /root/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/agent .

EXPOSE 8080

CMD ["./agent"]