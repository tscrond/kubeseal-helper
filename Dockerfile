ARG TARGETOS
ARG TARGETARCH

FROM golang:1.25.0-alpine3.21 AS builder

WORKDIR /kubeseal-helper

COPY . .

RUN go mod download

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /kubeseal-helper/kubeseal-helper /kubeseal-helper/

FROM golang:1.25.0-alpine3.21

WORKDIR /kubeseal-helper

COPY --from=builder /kubeseal-helper/kubeseal-helper /kubeseal-helper/kubeseal-helper

ENTRYPOINT ["/kubeseal-helper/kubeseal-helper"]
CMD []