FROM python:3 as python-scripts

RUN pip install --upgrade cx_Freeze

WORKDIR /src
COPY goodwe-python .

RUN cxfreeze -c gw.py --target-dir scripts

FROM golang:alpine as build

WORKDIR /app-build
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY *.go .

RUN go mod download && go mod verify
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/goodwe-metrics-exporter

FROM alpine

RUN apk update && apk add --no-cache gcompat

WORKDIR /app
COPY --from=python-scripts /src/scripts ./scripts
COPY --from=build /app .

RUN chmod +x scripts/gw goodwe-metrics-exporter

ENTRYPOINT [ "./goodwe-metrics-exporter" ]