FROM golang:1.24 as builder

WORKDIR /go/src/github.com/outrigger-project/autoscaler-expander-example

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -a -o manager main.go

FROM quay.io/centos/centos:stream9

WORKDIR /
COPY --from=builder /go/src/github.com/outrigger-project/autoscaler-expander-example/manager .

ENTRYPOINT ["/manager"]
