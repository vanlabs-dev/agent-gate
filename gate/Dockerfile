# The gate and the audit CLI in one image.
FROM golang:1.27.4 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ ./cmd/...

FROM scratch
COPY --from=build /out/agent-gate /out/agent-gate-audit /
USER 65532:65532
ENTRYPOINT ["/agent-gate"]
