FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /mailer ./services/mailer

FROM alpine:3.21
COPY --from=builder /mailer /mailer
CMD ["/mailer"]
