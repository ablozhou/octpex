# Stage 1: Build the Octpex biomimetic daemon binary
FROM golang:1.26.5-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY pkg/ pkg/
COPY internal/ internal/
COPY cluster/ cluster/
WORKDIR /src/cluster
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -ldflags="-s -w" -o /bin/octpex .

# Stage 2: Production runtime image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/octpex /usr/local/bin/octpex
COPY configs/ /app/configs/

EXPOSE 8080

ENTRYPOINT ["octpex"]
CMD ["run", "-profile=local", "-port=8080"]
