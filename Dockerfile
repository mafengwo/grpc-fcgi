FROM centos:7

ADD ./grpc_fastcgi_proxy /usr/local/bin/grpc_fastcgi_proxy

ADD ./conf/proxy.yml /etc/grpc_fastcgi_proxy/proxy.yml

ENTRYPOINT ["/usr/local/bin/grpc_fastcgi_proxy", "-f", "/etc/grpc_fastcgi_proxy/proxy.yml"]