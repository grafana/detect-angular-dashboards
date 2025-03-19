FROM golang:1.24-alpine3.21 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go build -v

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /app .

ENTRYPOINT ["./detect-angular-dashboards"]
