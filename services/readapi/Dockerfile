FROM golang:1.22 AS build

WORKDIR /src
COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/readapi ./cmd/readapi

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=build /out/readapi /readapi

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/readapi"]
