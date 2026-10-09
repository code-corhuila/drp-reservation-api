FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -H app
USER app
COPY --from=build /out/api /api
EXPOSE 8083
ENV HTTP_ADDR=:8083
ENTRYPOINT ["/api"]
