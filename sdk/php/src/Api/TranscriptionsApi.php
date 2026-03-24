<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Http\HttpClient;

class TranscriptionsApi
{
    /**
     * @readonly
     * @var HttpClient
     */
    private HttpClient $http;

    public function __construct(HttpClient $http)
    {
        $this->http = $http;
    }

    /**
     * List transcriptions for a session.
     *
     * @return array<array<string, mixed>>
     */
    public function listBySession(
        string $sessionId,
        ?int   $limit  = null,
        ?int   $offset = null
    ): array {
        $data = $this->http->get('/v1/transcriptions', [
            'session_id' => $sessionId,
            'limit'      => $limit,
            'offset'     => $offset,
        ]);

        return $data['items'] ?? $data;
    }
}
