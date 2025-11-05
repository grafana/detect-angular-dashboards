FROM golang:1.25-alpine3.21@sha256:0c9f3e09a50a6c11714dbc37a6134fd0c474690030ed07d23a61755afd3a812f AS build

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
