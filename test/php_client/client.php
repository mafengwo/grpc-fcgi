<?php

ini_set('display_errors', 'On');
error_reporting(E_ALL);

require_once dirname(__FILE__).'/vendor/autoload.php';

$point = new \Routeguide\Point();
$point->setLatitude($point->getLatitude())
    ->setLongitude($point->getLongitude());

$client = new \Routeguide\RouteGuideClient("localhost:50051", [
  'credentials' => Grpc\ChannelCredentials::createInsecure()
]);
$resp = $client->GetFeature($point)->wait();
var_dump($resp);
