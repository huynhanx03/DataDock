FROM golang:1.26-alpine AS build

WORKDIR /workspace/api

RUN apk add --no-cache git ca-certificates

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags='-s -w' -o /out/datadock-api ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/datadock-api /app/datadock-api

EXPOSE 8080

ENTRYPOINT ["/app/datadock-api"]
