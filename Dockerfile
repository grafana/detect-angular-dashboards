FROM golang:1.20.6-alpine3.18 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go build -v

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /app .

ENTRYPOINT ["./detect-angular-dashboards"]