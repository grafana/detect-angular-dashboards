FROM golang:1.24-alpine3.21@sha256:59a9237590705e00dd04bb43f9f75562caced1393b8767b4666a658daeb93e61 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go build -v

FROM alpine:3.21@sha256:5405e8f36ce1878720f71217d664aa3dea32e5e5df11acbf07fc78ef5661465b

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /app .

ENTRYPOINT ["./detect-angular-dashboards"]
