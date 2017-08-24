# grpc-fastcgi-proxy

Simple [grpc](http://www.grpc.io/)-to-fastcgi proxy.

## Status

This is little more than an experiment.  It works in the trivial [example](https://github.com/bakins/grpc-fastcgi-example) cases.

## Motivation

There is an official
[grpc client](http://www.grpc.io/docs/tutorials/basic/php.html)
for PHP, but no server support.  At my dayjob, we use a lot of PHP, so I wanted
to experiment with gRPC servers in PHP.

This project is only the proxy. There is [another repo](https://github.com/bakins/grpc-fastcgi-example) with an example PHP
application.  That example is used in the testing of this project.

## Building/Installing

You need a working [Go installation](https://golang.org/doc/install#install).

Clone this repository into your GOPATH and build the command. For example:

```shell
cd $HOME
mkdir -p go/src/github.com/bakins
cd go/src/github.com/bakins
git clone github.com/bakins/grpc-fastcgi-proxy
cd grpc-fastcgi-proxy
go build ./cmd/grpc-fastcgi-proxy
```

You should now have a `grpc-fastcgi-proxy` binary

## Usage

```shell
$ ./grpc-fastcgi-proxy --help
grpc to fastcgi proxy

Usage:
  grpc-fastcgi-proxy [flags]

Flags:
  -a, --address string   listen address (default "127.0.0.1:8080")
  -f, --fastcgi string   fastcgi to proxy (default "127.0.0.1:9000")
  -h, --help             help for grpc-fastcgi-proxy
```

`grpc-fastcgi-proxy` is intended to be used with single entrypoint applications.
You should do all your routing in `index.php`, for example.  This entry file should 
be passed as an argument to `grpc-fastcgi-proxy`:

```shell
$ ./grpc-fastcgi-proxy $HOME/git/grpc-fastcgi-example/index.php
```

It will set the `SCRIPT_FILE` and `DOCUMENT_ROOT` cgi variables.

## TODO

- general code cleanup
- convert HTTP status codes to corresponding gRPC error codes
- add timeouts to the [fcgi client](https://github.com/kellegous/fcgi)

## License

See [LICENSE](./LICENSE)

## Acknowledgements

- [Kelly](https://github.com/kellegous) for the [fastcgi](https://github.com/kellegous/fcgi) code and examples and for not talking me out of doing this.
- [Michal Witkowski](https://github.com/mwitkow) for the [grpc-proxy](https://github.com/mwitkow/grpc-proxy).
