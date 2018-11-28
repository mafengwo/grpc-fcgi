<?php
ini_set('display_errors', 'On');
error_reporting(E_ALL);

require_once dirname(__FILE__).'/vendor/autoload.php';

$body = file_get_contents("php://input");

$request = new \Flight\Price\CityCheapestPriceRequest();
$request->mergeFromString($body);

// echo "departure_id:", $request->getDepartureCityID();
// echo "arrive_id:", $request->getArriveCityID();

$response = new \Flight\Price\CityCheapestPriceReply();
$response->setDepartureCityID($request->getDepartureCityID());
$response->setArriveCityID($request->getArriveCityID());
$response->setPrice("10.1");
echo $response->serializeToString();
