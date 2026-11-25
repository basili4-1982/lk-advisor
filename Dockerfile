FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/lk-advisor ./cmd/lk-advisor


FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=build /out/lk-advisor /usr/local/bin/lk-advisor

USER nobody

EXPOSE 8080

ENTRYPOINT ["lk-advisor"]
