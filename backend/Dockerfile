FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /bin/server /bin/server

EXPOSE 8000
CMD ["/bin/server"]
