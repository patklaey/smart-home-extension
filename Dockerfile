ARG GOVERSION=1.24.4

FROM golang:${GOVERSION} AS compile

WORKDIR /app

COPY go.mod go.sum main.go ./
COPY internal ./internal/

RUN go build -o /app/goapp

FROM golang:${GOVERSION} AS application

COPY --from=compile /app/goapp /app/goapp

EXPOSE 8080
EXPOSE 8088
ENTRYPOINT [ "/app/goapp" ]