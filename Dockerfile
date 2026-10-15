# ---- build ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web/ ./web/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/qingimage .

# ---- runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H app \
 && mkdir -p /data && chown app:app /data
COPY --from=build /out/qingimage /usr/local/bin/qingimage
USER app
ENV IMG_DATA_DIR=/data IMG_PORT=8080 TZ=Asia/Shanghai
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["qingimage"]
