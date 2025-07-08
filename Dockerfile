FROM registry.cn-shanghai.aliyuncs.com/lesroad/infrastructure:golang_1.23-alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct

# 修改 Alpine 包管理工具 apk 的软件源为阿里云镜像
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
RUN sh ./build.sh

FROM registry.cn-shanghai.aliyuncs.com/lesroad/infrastructure:alpine_3.18

COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /build/output /app

CMD ["sh", "./bootstrap.sh"]