FROM php:7.2.10-fpm-alpine

RUN docker-php-ext-install pdo_mysql

RUN apk --no-cache add tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    apk del tzdata
