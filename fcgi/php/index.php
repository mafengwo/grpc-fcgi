<?php
ini_set('display_errors', 'On');
error_reporting(E_ALL);

require_once dirname(__FILE__).'/vendor/autoload.php';

$body = file_get_contents("php://input");

$request = new \Flight\Price\CityCheapestPriceRequest();
$request->mergeFromString($body);

// echo "departure_id:", $request->getDepartureCityID();
// echo "arrive_id:", $request->getArriveCityID();

$arriveId = $request->getArriveCityID();
$depId = $request->getDepartureCityID();
header("arrive_id: $arriveId");
header("dep_id: $depId");
header("body_len: ".strlen($body));
header("pid: ".getmypid());
header("request_content_length: ". $_SERVER['CONTENT_LENGTH']);
header("request_user_agent: ". $_SERVER['HTTP_USER_AGENT']);

$response = new \Flight\Price\CityCheapestPriceReply();
$response->setDepartureCityID($depId);
$response->setArriveCityID($arriveId);
$response->setPrice("10.1");
echo $response->serializeToString();
