FROM golang:1.26-trixie@sha256:bbf22ddccb3205344f2755ea8fa4fe39f7a8b2b77b9f7b764ec2aad31406f6fc AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/external-dns-yandex-webhook ./cmd/webhook

FROM gcr.io/distroless/static-debian13:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240

COPY --from=builder /out/external-dns-yandex-webhook /external-dns-yandex-webhook
ENTRYPOINT ["/external-dns-yandex-webhook"]
