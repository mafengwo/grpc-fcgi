# gRPC 转 fastcgi 的代理工具

本项目是提供一个代理工具，将gRPC请求转换成fastcgi请求，
然后再将fastcgi的结果以gRPC通讯协议发送给gRPC客户端。

## 项目背景

我们需要使用PHP作为微服务的server端，并且通讯协议期望是gRPC。
遗憾的是gRPC官方并没有支持PHP Server。当我们了解gRPC的通讯协议是基于HTTP2之后，
我们曾试图通过Nginx来作为WebServer(因为Nginx支持HTTP2)。
但问题是：Nginx并不完全满足gRPC的通讯协议，这会导致比较大的局限性。(技术细节在此暂不细述)
考虑到微服务的重要性，这些局限性从长期来看是不被接受的。

所以我们提出自行研发一个WebServer，这就是这个项目的来源。

## 功能介绍

从逻辑上来说，gRPC请求分为两部分 metadata 和 body。下面分别介绍。

#### metadata 与 fastcgi 参数
本工具会将metadata部分以fastcgi参数的形式传入到PHP, 但会进行简单地转换：
1. 将metadata的key全部转成大写。
2. 将key中的"-"转成"_"
3. 在key的前面附上 HTTP_

除了metadata中的key之外，fastcgi参数中还会包括：
1. CONTENT_TYPE 取值统一为 "application/grpc"
2. REQUEST_URI  请求的URI
3. DOCUMENT_ROOT 程序目录
4  SCRIPT_FILENAME 执行脚本地址

在PHP中，以上fastcgi参数存放在 $_SERVER 中。

#### body
gRPC请求的body是按照protobuf协议进行编码的二进制内容。这些内容会被透传到fastcgi服务端。
也就是说，fastcgi服务端需要按照protobuf协议解码后方能使用这些数据。

同样地，fastcgi端也需要将结果先通过protobuf编码再返回，这样gRPC客户端才能解析。

protobuf提供了PHP的编解码函数，在此不具体说明了。

需要注意的是：
1. protobuf同时提供了PHP的扩展和SDK。请使用PHP扩展，因为PHP的SDK的编解码速度很慢。
2. 在PHP中，无法再通过$_GET, $_POST, $_REQUEST, $_COOKIE, $_FILE, $_SESSION, $_ENV这些变量中获取数据。只有$_SERVER变量会被启用。
3. body存放在php://input中。

## 使用说明

### 运行程序
在项目根目录下运行下面命令生成可执行文件。
make build-darwin (for mac) 或
make build-linux (for linux)

在工程目录下执行
bin/grpc_fastcgi_proxy_darwin -f conf/proxy.yml (for mac) 或
bin/grpc_fastcgi_proxy_linux -f conf/proxy.yml (for linux)

### 配置项说明

配置文件路径为 conf/proxy.yml

配置项示例与说明

```
# 该工具的监听地址
address: "0.0.0.0:8080"

# 每次请求的处理超时时间。单位：秒
# 如果超时，会返回 Deadline exceeded 错误（grpc的错误码为4）
timeout: 10

# pprof 工具的服务地址
# 我们可以通过访问路径/debug/pprof, 获知该代理程序的运行状态（内存占用，goroutine数量等）
pprof_address: "0.0.0.0:9876"

fastcgi:
  # 转发的fastcgi地址
  address: "127.0.0.1:9000"
  # 最大链接数
  max_connections: 300
  # 最大空闲链接数
  max_idle_connections: 100
  # fastcgi param SCRIPT_FILENAME
  script_file_name: "/opt/fcgi/php/index.php"
  # fastcgi param DOCUMENT_ROOT
  document_root: "/opt/fcgi/php/"
# 说明：
# 1. 所有日志格式均为json
log:
  # 访问日志的保存路径
  # 这里可以填写 stdout 或 stderr 来代表标准输出和标准错误输出
  # 日志字段包括：
  # request_id: 请求ID。这个值可以在grpc请求的metadata中设置（key为request_id）.
  #             如果metadata中缺失，则自动生成。
  # time: 接收到grpc stream的时间
  # host: grpc请求的host，client发送grpc请求时，会将该值设置在metadata的:authority中
  # request_uri: 请求地址
  # request_length: 请求的grpc frame中payload大小，不含header。
  # round_trip_time: 从接收grpc请求到将结果返回给client(准确来说是：写入到网络层)的耗时。单位（秒）
  # upstream_time: fastcgi处理结果的时间(从写入完成到接收到第一个字节)。 单位（秒）
  # status: grpc状态码
  # upstream_status: fastcgi返回的状态码。 如果没有返回状态码，则是200.
  # body_bytes_sent: 响应的grpc frame中payload大小，不含header
  #
  # 说明:
  # 1. 因为本工具采用了uber的zap logsdk。该sdk会为每条日志默认加上三个字段：level, ts, msg。
  #    这三个字段对于访问日志来所是多余的，后续会去掉这三个字段（ts可能保留）。
  access_log_path: "stdout"
  # 是否开启访问日志的详细模式
  # 当开启详细模式后，访问字段中会增加几个字段，帮助分析fastcgi连接的各项耗时。字段包括
  # GetConn: 开始获取fastcgi连接的时间点
  # GotConn: 获取到连接的时间点
  # ConnectStart： 开始建立连接（拨号）的时间点。如果是复用连接，则没有该字段。
  # ConnectDone: 拨号完成的时间点
  # WroteHeaders: 将请求的headers写入到网络层的时间点
  # WroteRequest: 将整个请求写入到网络层的时间点
  # GotFirstResponseByte: 读到第一个响应字节的时间点
  # PutIdle: 请求处理完成之后，将连接放入到空闲池的时间点
  # PutIdleError: 未成功放入空闲次的错误原因（通常是空闲池大小超过设置项）
  access_log_verbose: true
  # 错误日志的保存路径
  error_log_path: "stderr"
  # 错误日志的最低写入级别。错误级别由高到低为： error； warn； info; debug;
  error_log_level: "info"
```
## 日志说明
## 实现原理