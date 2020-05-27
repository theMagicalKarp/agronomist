FROM golang:1.13-alpine as build

ENV CGO_ENABLED=0

WORKDIR /build

COPY go.mod go.sum /build/
RUN go mod download

COPY . /build

RUN go build -o /agronomist .


FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=build /agronomist .

USER nonroot:nonroot

ENTRYPOINT ["/agronomist"]
