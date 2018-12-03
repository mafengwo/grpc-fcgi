FROM docker.mfwdev.com/paas/php-fpm:5.6.36-fpm.v0.1.9

ADD ./bin/grpc-proxy_unix /usr/local/bin/grpc-proxy
ADD ./conf/proxy.yml /etc/grpc-proxy.yml

ENTRYPOINT ["docker-php-entrypoint", "-R"]