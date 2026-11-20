FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/events ./cmd/events

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build --chown=65532:65532 /out/events /events
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/events"]
