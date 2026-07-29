FROM golang:1.25-alpine3.21@sha256:b4dbd292a0852331c89dfd64e84d16811f3e3aae4c73c13d026c4d200715aff6 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go build -v

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /app .

ENTRYPOINT ["./detect-angular-dashboards"]
