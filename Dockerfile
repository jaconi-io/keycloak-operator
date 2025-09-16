FROM golang:1.25 AS builder

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o keycloak-operator ./cmd

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /workspace/keycloak-operator /

USER 65532:65532

ENTRYPOINT ["/keycloak-operator"]
