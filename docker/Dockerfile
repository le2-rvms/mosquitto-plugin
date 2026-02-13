FROM debian:trixie-slim AS build-main

ENV VERSION=2.1.2 \
    DOWNLOAD_SHA256=fd905380691ac65ea5a93779e8214941829e3d6e038d5edff9eac5fd74cbed02 \
    GPG_KEYS=A0D6EEA1DCAE49A635A3B2F0779B22DFB3E717B7

ARG APP_ENV=prod

RUN set -eux; \
    if [ "${APP_ENV:-}" = "dev" ]; then \
    sed -i "s|http://deb.debian.org|http://mirrors.aliyun.com|g" /etc/apt/sources.list.d/debian.sources; \
    fi

# 构建依赖（使用 Debian 组件提供的 libwebsockets-dev）
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    gnupg \
    libargon2-dev \
    libcjson-dev \
    libedit-dev \
    libmicrohttpd-dev \
    libssl-dev \
    libsqlite3-dev \
    wget \
    ca-certificates \
    ; \
    rm -rf /var/lib/apt/lists/*

# 下载 + 校验 Mosquitto 源码（sha256 + GPG）
RUN set -eux; \
    wget "https://mosquitto.org/files/source/mosquitto-${VERSION}.tar.gz" -O /tmp/mosq.tar.gz; \
    echo "$DOWNLOAD_SHA256  /tmp/mosq.tar.gz" | sha256sum -c -; \
    wget "https://mosquitto.org/files/source/mosquitto-${VERSION}.tar.gz.asc" -O /tmp/mosq.tar.gz.asc; \
    export GNUPGHOME="$(mktemp -d)"; \
    found=''; \
    for server in \
    hkps://keys.openpgp.org \
    hkp://keyserver.ubuntu.com:80 \
    pgp.mit.edu \
    ; do \
    echo "Fetching GPG key $GPG_KEYS from $server"; \
    gpg --keyserver "$server" --keyserver-options timeout=10 --recv-keys "$GPG_KEYS" && found=yes && break; \
    done; \
    test -z "$found" && echo >&2 "error: failed to fetch GPG key $GPG_KEYS" && exit 1; \
    gpg --batch --verify /tmp/mosq.tar.gz.asc /tmp/mosq.tar.gz; \
    gpgconf --kill all; \
    rm -rf "$GNUPGHOME" /tmp/mosq.tar.gz.asc; \
    mkdir -p /build/mosq; \
    tar --strip=1 -xf /tmp/mosq.tar.gz -C /build/mosq; \
    rm /tmp/mosq.tar.gz

# 编译并安装到 /out（独立根目录）
RUN set -eux; \
    mkdir -p /out; \
    make -C /build/mosq -j"$(nproc)" \
    CFLAGS="-Wall -O2 -I/build -DHTTP_API_DIR=\\\"/usr/share/mosquitto/dashboard\\\"" \
    WITH_ADNS=no \
    WITH_DOCS=no \
    WITH_SHARED_LIBRARIES=yes \
    WITH_SRV=no \
    WITH_STRIP=yes \
    WITH_WEBSOCKETS=yes \
    prefix=/usr \
    binary \
    ; \
    # 一次性安装到 /out（不污染 builder 的 /usr）
    make -C /build/mosq \
    WITH_ADNS=no WITH_DOCS=no WITH_SHARED_LIBRARIES=yes WITH_SRV=no WITH_STRIP=yes WITH_WEBSOCKETS=yes \
    prefix=/usr DESTDIR=/out install \
    ; \
    # 准备默认配置以兼容 /mosquitto/config 路径
    mkdir -p /out/mosquitto/config; \
    install -m644 /build/mosq/docker/2.1-ubuntu/mosquitto.conf /out/mosquitto/config/mosquitto.conf; \
    install -m644 /build/mosq/docker/2.1-ubuntu/mosquitto.conf /out/mosquitto-no-auth.conf; \
    install -d /out/usr/share/mosquitto; \
    cp -r /build/mosq/dashboard/src /out/usr/share/mosquitto/dashboard; \
    install -Dm644 /build/mosq/epl-v20 /out/usr/share/licenses/mosquitto/epl-v20; \
    install -Dm644 /build/mosq/edl-v10 /out/usr/share/licenses/mosquitto/edl-v10



# Multi-stage build: build plugin .so then run with mosquitto
FROM golang:latest AS build-plugin

ARG APP_ENV=prod

RUN set -eux; \
    if [ "${APP_ENV:-}" = "dev" ]; then \
    sed -i "s|http://deb.debian.org|http://mirrors.aliyun.com|g" /etc/apt/sources.list.d/debian.sources; \
    fi

RUN set -eux; \
    apt-get update ;\
    apt-get install -y --no-install-recommends \
    build-essential pkg-config ca-certificates \
    libcjson-dev \
    ; \
    rm -rf /var/lib/apt/lists/*

# 使用 build-main 产出的 mosquitto headers/libs
COPY --from=build-main /out/usr/include/ /usr/local/include/
COPY --from=build-main /out/usr/lib/ /usr/local/lib/

ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig \
    LD_LIBRARY_PATH=/usr/local/lib

WORKDIR /src
COPY go.mod .
RUN go mod download
COPY . .
RUN make build-auth build-queue build-conn


# https://packages.debian.org/search?keywords=mosquitto
FROM debian:trixie-slim

ARG APP_ENV=prod

RUN set -eux; \
    if [ "${APP_ENV:-}" = "dev" ]; then \
    sed -i "s|http://deb.debian.org|http://mirrors.aliyun.com|g" /etc/apt/sources.list.d/debian.sources; \
    fi

RUN set -eux; \
    apt-get update ;\
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    libargon2-1 \
    libcjson1 \
    libedit2 \
    libmicrohttpd12 \
    libsqlite3-0 \
    ; \
    rm -rf /var/lib/apt/lists/*

# 运行用户与目录
# RUN set -eux; \
#     groupadd -r -g 1883 mosquitto || true; \
#     useradd  -r -u 1883 -g mosquitto -d /var/empty -s /usr/sbin/nologin -c "mosquitto" mosquitto || true; \
#     mkdir -p /mosquitto/config /mosquitto/data /mosquitto/log; \
#     chown -R mosquitto:mosquitto /mosquitto

RUN set -eux; \
    groupadd -r -g 1883 mosquitto || true; \
    useradd  -r -u 1883 -g mosquitto -d /var/empty -s /usr/sbin/nologin -c "mosquitto" mosquitto || true;
# mkdir -p /mosquitto/config /mosquitto/data /mosquitto/log; \
# chown -R mosquitto:mosquitto /mosquitto

# Copy plugin into the image
COPY --from=build-plugin --chown=mosquitto:mosquitto /src/build/ /mosquitto/plugins/

# USER mosquitto

# 拷贝构建产物
COPY --from=build-main /out/ /

VOLUME ["/mosquitto/data", "/mosquitto/log"]

COPY ./docker/docker-entrypoint.sh /
EXPOSE 1883
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/usr/sbin/mosquitto", "-c", "/mosquitto/config/mosquitto.conf"]
# CMD ["mosquitto"] 
