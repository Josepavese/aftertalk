<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Http\HttpClient;

class TranscriptionsApi
{
    public function __construct(private readonly HttpClient $http) {}

    /**
     * List transcriptions for a session.
     *
     * @return array<array<string, mixed>>
     */
    public function listBySession(
        string $sessionId,
        ?int   $limit  = null,
        ?int   $offset = null,
    ): array {
        $data = $this->http->get('/v1/transcriptions', [
            'session_id' => $sessionId,
            'limit'      => $limit,
            'offset'     => $offset,
        ]);

        return $data['items'] ?? $data;
    }
}
