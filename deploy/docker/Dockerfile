# Stage 1: Build the Octpex biomimetic daemon binary
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/octpex cmd/octpex/main.go

# Stage 2: Production runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/octpex /usr/local/bin/octpex
COPY configs/ /app/configs/

EXPOSE 8080

ENTRYPOINT ["octpex"]
CMD ["run", "-profile=local", "-port=8080"]
