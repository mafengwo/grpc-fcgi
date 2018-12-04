<?php
ini_set('display_errors', 'On');
error_reporting(E_ALL);

require_once dirname(__FILE__).'/vendor/autoload.php';

$body = file_get_contents("php://input");

list(, $method) = explode("/", trim($_SERVER['REQUEST_URI'], "/"));
switch($method) {
    case "GetFeature":
        $resp = get_feature($body);
        break;
    case "ListFeatures":
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