<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Dto\RtcConfig;
use Aftertalk\Dto\ServerConfig;
use Aftertalk\Http\HttpClient;

class ConfigApi
{
    public function __construct(private readonly HttpClient $http) {}

    /**
     * Returns server configuration: available templates, STT and LLM profiles,
     * and their respective defaults.
     */
    public function getConfig(): ServerConfig
    {
        $data = $this->http->get('/v1/config');
        return ServerConfig::fromArray($data);
    }

    /**
     * Returns ICE server list for WebRTC (STUN/TURN).
     * Public endpoint — no API key required.
     */
    public function getRtcConfig(): RtcConfig
    {
        $data = $this->http->get('/v1/rtc-config');
        return RtcConfig::fromArray($data);
    }
}
