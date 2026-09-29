FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ost ./cmd/ost

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /ost /ost
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["/ost"]
CMD ["server", "--addr", ":8081"]
