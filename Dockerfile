FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notesmcp ./cmd/notesmcp

FROM alpine:3.22
RUN adduser -D -H app
USER app
COPY --from=build /out/notesmcp /usr/local/bin/notesmcp
EXPOSE 8080
ENV NOTES_MCP_ADDR=0.0.0.0:8080
ENTRYPOINT ["notesmcp"]

