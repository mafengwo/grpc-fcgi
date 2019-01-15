<?php
ini_set('display_errors', 'On');
error_reporting(E_ALL);

require_once dirname(__FILE__).'/vendor/autoload.php';

$body = file_get_contents("php://input");

list(, $method) = explode("/", trim($_SERVER['REQUEST_URI'], "/"));
switch($method) {
    case "GetFeature":
        $resp = get_feature($body);
        header("grpc-status: 12");
        header("grpc-message: message");
        header("test-header:".json_encode($_SERVER));
        header("trailer-a: ta");
        header("HTTP/1.1 404 Not Found");
        break;

    case "ListFeatures":
        break;
    case "RecordRoute":
        $resp = get_feature($body);
        break;
    default:
        throw new \Exception("unknown method");
}

echo $resp->serializeToString();

function get_feature($body) {
    $point = new \Routeguide\Point();
    $point->mergeFromString($body);

    $resp = new \Routeguide\Feature();

    $respPoint = new \Routeguide\Point();
    $respPoint->setLatitude($point->getLatitude())
        ->setLongitude($point->getLongitude());

    return $resp->setName("feature_name")
        ->setLocation($respPoint);
}

function record_route($body) {
    $point = new \Routeguide\Point();
    $point->mergeFromString($body);

    $resp = new \Routeguide\Feature();

    $respPoint = new \Routeguide\Point();
    $respPoint->setLatitude($point->getLatitude())
        ->setLongitude($point->getLongitude());

    return $resp->setName("feature_name")
        ->setLocation($respPoint);
}