FROM ubuntu:20.04

MAINTAINER "TruthHun <TruthHun@QQ.COM>"

ENV DEBIAN_FRONTEND=noninteractive
ENV LANG en_US.utf8

# 替换为国内源以加速构建
RUN sed -i s@/archive.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list \
    && sed -i s@/security.ubuntu.com/@/mirrors.aliyun.com/@g /etc/apt/sources.list \
    && apt clean \
    && apt update -y \
    && apt install -y locales \
    && rm -rf /var/lib/apt/lists/* \
    && localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8 \
    && apt update -y \
    && apt install -y fonts-wqy-zenhei fonts-wqy-microhei \
    && apt install -y xdg-utils wget xz-utils python3 chromium-browser netcat \
    && wget -nv -O- https://download.calibre-ebook.com/linux-installer.sh | sh /dev/stdin version=5.44.0

# 将程序拷贝进去
COPY . /www/BookStack/

# 赋予执行权限
RUN chmod 0755 -R /www/BookStack/

WORKDIR /www/BookStack/

# CMD 已在 docker-compose 中覆盖，保留默认CMD作为备选
CMD [ "./BookStack" ]