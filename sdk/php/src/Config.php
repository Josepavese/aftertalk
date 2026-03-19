<?php

declare(strict_types=1);

namespace Aftertalk;

class Config
{
    /**
     * @param string      $baseUrl        Aftertalk server base URL (no trailing slash)
     * @param string      $apiKey         API key for all /v1/* endpoints
     * @param string|null $webhookSecret  HMAC-SHA256 secret for webhook verification
     * @param int         $timeout        HTTP request timeout in seconds (default: 30)
     */
    public function __construct(
        public readonly string $baseUrl,
        public readonly string $apiKey,
        public readonly ?string $webhookSecret = null,
        public readonly int $timeout = 30,
    ) {}
}
