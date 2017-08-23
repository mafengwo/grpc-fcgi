<?php
// this would be in a library that should be included via composer

class GRPCCallInfo {
  private $service;
  private $method;

  function __construct(string $service, string $method) {
    $this->service = $service;
    $this->method = $method;
  }

  public function getService() : string {
    return $this->service;
  }

  public function getMethod() : string {
    return $this->method;
  }
}

interface GRPCInterceptor {
  // interceptor should just call_user_func($handler, $arg)
  public function intercept(GRPCCallInfo $info, $handler, $arg);
}

class GRPCServer {
  private $interceptors = [];
  private $mapping = [];

  function __construct() {
  }

  public function AddInterceptor(GRPCInterceptor $i) {
    array_push($this->interceptors, $i);
  }

  public function RegisterServer(string $name, $callbacks) {
    // register route for server /$name/*
    $this->mapping[$name] = $callbacks;
  }

  public function Serve($path = null) {
    // if $path is null, extract it from global
    if (is_null($path)) {
      $path = $_SERVER['REQUEST_URI'] ?? null;
      // if $path is still empty, thrown an error?
    }

    // should trigger only on POST?

    $path = ltrim($path, "/");
    $path = rtrim($path, "/");

    $parts = explode("/", $path, 2);
    // if len $parts is != 2, then return an error

    $serviceName = $parts[0];
    $methodName = $parts[1];

    $service = $this->mapping[$serviceName];
    // handle error if not found
    $method = $service[$methodName];
    // handle error if not found

    $body = file_get_contents('php://input');
    //$f = fopen('php://input', 'b');
    //$body = fread($f, 8192);
    //fclose($f);
    // call any inteceptors

    // TODO: handle errors

    //$response = '';
    // catch any errors
    //if (count($this->interceptors) == 0) {
    $response = call_user_func($method, $body);

    //} else {
    //  $info = new \GRPCCallInfo($serviceName, $methodName);
    //  $response = $body;
      //foreach($this->interceptors as $item) {
    //    $response = call_user_func(array($item, 'intercept'), $info, $method, $response);
    //  }
    //}

    // set headers, including "Special" grpc headers that will be turned into trailers.
    // or just let helper do it?
    //header('Content-Type:  application/grpc+proto');
    //var_dump($response);
    print($response);
  }
}

?>
