<?php

declare(strict_types=1);

namespace Aftertalk;

class Config
{
    /**
     * @readonly
     * @var string Aftertalk server base URL (no trailing slash)
     */
    public string $baseUrl;

    /**
     * @readonly
     * @var string API key for all /v1/* endpoints
     */
    public string $apiKey;

    /**
     * @readonly
     * @var string|null HMAC-SHA256 secret for webhook verification
     */
    public ?string $webhookSecret;

    /**
     * @readonly
     * @var int HTTP request timeout in seconds (default: 30)
     */
    public int $timeout;

    /**
     * @param string      $baseUrl        Aftertalk server base URL (no trailing slash)
     * @param string      $apiKey         API key for all /v1/* endpoints
     * @param string|null $webhookSecret  HMAC-SHA256 secret for webhook verification
     * @param int         $timeout        HTTP request timeout in seconds (default: 30)
     */
    public function __construct(
        string  $baseUrl,
        string  $apiKey,
        ?string $webhookSecret = null,
        int     $timeout       = 30
    ) {
        $this->baseUrl       = $baseUrl;
        $this->apiKey        = $apiKey;
        $this->webhookSecret = $webhookSecret;
        $this->timeout       = $timeout;
    }
}
