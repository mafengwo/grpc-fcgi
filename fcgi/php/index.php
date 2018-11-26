<?php
// Full gRPC method name in format:
// package.service-name/method-name
ini_set('display_errors', 'On');
error_reporting(E_ALL);

ob_start();

require_once dirname(__FILE__).'/vendor/autoload.php';

//throw new \Exception("test");
$a = $_GET['r']
echo print_r($_GET, true);
// Protobuf-serialized request message body
$body = file_get_contents("php://input");
exit;
try {
    // We can throw some errors here...
    if (rand(0, 1)) {
        throw new \RuntimeException("Some error happened!");
    }

    echo "1";
    // Send response to the client
    //echo $response->serialize();
} catch (\Throwable $e) {
    echo json_encode([
        'error' => $e->getMessage()
    ]);
}
$buf = ob_get_clean();
$errfile = dirname(__FILE__)."/error.txt";
file_put_contents($errfile, $errorInfo);
echo $buf;
exit;
