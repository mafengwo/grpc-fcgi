FROM centos:7

ADD ./bin/grpc_fastcgi_proxy_linux /usr/local/bin/grpc_fastcgi_proxy

ADD ./conf/proxy.yml /etc/grpc_fastcgi_proxy/proxy.yml

ENTRYPOINT ["/usr/local/bin/grpc_fastcgi_proxy", "-f", "/etc/grpc_fastcgi_proxy/proxy.yml"]