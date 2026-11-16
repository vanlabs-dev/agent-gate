# One image holds both stub binaries. The manifests select one by command.
FROM golang:1.27.4 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -o /out/ ./cmd/...

FROM scratch
COPY --from=build /out/stub-gate /out/stub-mcp /
USER 65532:65532
