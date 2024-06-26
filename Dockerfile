ARG GOVERSION=1.22.4

FROM golang:${GOVERSION} as compile

WORKDIR /app

COPY go.mod go.sum main.go ./
COPY internal ./internal/

RUN go build -o /app/goapp

FROM golang:${GOVERSION} as application

COPY --from=compile /app/goapp /app/goapp

EXPOSE 8080
EXPOSE 8088
ENTRYPOINT [ "/app/goapp" ]