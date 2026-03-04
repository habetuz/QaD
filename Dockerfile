FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o qad .

FROM scratch

COPY --from=builder /app/qad /qad

EXPOSE 8080 9876

ENTRYPOINT ["/qad"]
