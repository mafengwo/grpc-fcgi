# gRPC 转 fastcgi 的代理工具

本项目是提供一个代理工具，将gRPC请求转换成fastcgi请求，
然后再将fastcgi的结果以gRPC通讯协议发送给gRPC客户端。

本工具只负责协议转换与发送，不负责对传输的数据内容(body)进行转换。
也就是说，fastcgi端接收到的数据依旧是经过protobuf编码的字节流。
fastcgi端也需要将结果先通过protobuf编码再返回，这样gRPC客户端才能解析。

## 项目背景

我们需要使用PHP作为微服务的server端，并且通讯协议期望是gRPC。
遗憾的是gRPC官方并没有支持PHP Server。当我们了解gRPC的通讯协议是基于HTTP2之后，
我们曾试图通过Nginx来作为WebServer，因为Nginx支持HTTP2。
但同样遗憾的是，Nginx并不完全满足gRPC的通讯协议，这会导致比较大的局限性。
考虑到微服务的重要性，这些局限性从长期来看是不被接受的。

所以我们提出自行研发一个WebServer，这就是这个项目的来源。

## 功能介绍


## 使用说明

## 实现原理