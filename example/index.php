<?php
require_once __DIR__ . '/vendor/autoload.php';
require_once __DIR__ . '/helloworld.pb.php';

$body = file_get_contents('php://input');

$request = new \Helloworld\HelloRequest();
$request->mergeFromString($body);

xxx


$response = new \Helloworld\HelloReply();
$response->setMessage(sprintf("Hello %s", $request->getName()));

header("HTTP/1.1 404 Not Found");
header('Content-Type: application/grpc');

print($response->serializeToString());
 
