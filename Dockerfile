FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.serverVersion=${VERSION}" \
    -o /out/temp-email-mcp .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/temp-email-mcp /temp-email-mcp
EXPOSE 8080
ENTRYPOINT ["/temp-email-mcp"]
CMD ["--transport", "http", "--addr", "0.0.0.0:8080"]
