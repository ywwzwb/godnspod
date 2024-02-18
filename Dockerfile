FROM golang:1.22-alpine
# this run command is for speeding up in China
# RUN go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct && \
#     sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
RUN apk update && \
    apk add git && \
    go get gopkg.in/yaml.v2

WORKDIR /go/src/godnspod/
COPY ./ ./
RUN go build

FROM alpine:latest
COPY --from=0 /go/src/godnspod/godnspod /bin/godnspod
COPY --from=0 /go/src/godnspod/config.yaml /config/config.yaml
CMD [ "/bin/godnspod", "-config_path", "/config/config.yaml" ]
