FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN apk --no-cache add ca-certificates curl \
    && curl -fsSL -o /usr/local/share/ca-certificates/russian_trusted_root_ca.crt \
       https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt \
    && update-ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations
USER app

CMD ["/app/api"]
