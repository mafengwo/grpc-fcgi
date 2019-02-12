<?php
// GENERATED CODE -- DO NOT EDIT!

namespace Routeguide;

/**
 * Interface exported by the server.
 */
class RouteGuideClient extends \Grpc\BaseStub {

    /**
     * @param string $hostname hostname
     * @param array $opts channel options
     * @param \Grpc\Channel $channel (optional) re-use channel object
     */
    public function __construct($hostname, $opts, $channel = null) {
        parent::__construct($hostname, $opts, $channel);
    }

    /**
     * A simple RPC.
     *
     * Obtains the feature at a given position.
     *
     * A feature with an empty name is returned if there's no feature at the given
     * position.
     * @param \Routeguide\Point $argument input argument
     * @param array $metadata metadata
     * @param array $options call options
     */
    public function GetFeature(\Routeguide\Point $argument,
      $metadata = [], $options = []) {
        return $this->_simpleRequest('/routeguide.RouteGuide/GetFeature',
        $argument,
        ['\Routeguide\Feature', 'decode'],
        $metadata, $options);
    }

    /**
     * A server-to-client streaming RPC.
     *
     * Obtains the Features available within the given Rectangle.  Results are
     * streamed rather than returned at once (e.g. in a response message with a
     * repeated field), as the rectangle may cover a large area and contain a
     * huge number of features.
     * @param \Routeguide\Rectangle $argument input argument
     * @param array $metadata metadata
     * @param array $options call options
     */
    public function ListFeatures(\Routeguide\Rectangle $argument,
      $metadata = [], $options = []) {
        return $this->_serverStreamRequest('/routeguide.RouteGuide/ListFeatures',
        $argument,
        ['\Routeguide\Feature', 'decode'],
        $metadata, $options);
    }

    /**
     * A client-to-server streaming RPC.
     *
     * Accepts a stream of Points on a route being traversed, returning a
     * RouteSummary when traversal is completed.
     * @param array $metadata metadata
     * @param array $options call options
     */
    public function RecordRoute($metadata = [], $options = []) {
        return $this->_clientStreamRequest('/routeguide.RouteGuide/RecordRoute',
        ['\Routeguide\RouteSummary','decode'],
        $metadata, $options);
    }

    /**
     * A Bidirectional streaming RPC.
     *
     * Accepts a stream of RouteNotes sent while a route is being traversed,
     * while receiving other RouteNotes (e.g. from other users).
     * @param array $metadata metadata
     * @param array $options call options
     */
    public function RouteChat($metadata = [], $options = []) {
        return $this->_bidiRequest('/routeguide.RouteGuide/RouteChat',
        ['\Routeguide\RouteNote','decode'],
        $metadata, $options);
    }

}
