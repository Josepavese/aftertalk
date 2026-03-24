<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Dto\Minutes;
use Aftertalk\Http\HttpClient;

class MinutesApi
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

    /** Get the minutes for a session. */
    public function getBySession(string $sessionId): Minutes
    {
        $data = $this->http->get('/v1/minutes', ['session_id' => $sessionId]);
        return Minutes::fromArray($data);
    }

    /** Get minutes by their own ID. */
    public function get(string $minutesId): Minutes
    {
        $data = $this->http->get("/v1/minutes/{$minutesId}");
        return Minutes::fromArray($data);
    }

    /**
     * Update minutes sections (saves previous version to history).
     *
     * @param array<string, mixed> $sections
     */
    public function update(
        string  $minutesId,
        array   $sections,
        ?string $notes  = null,
        ?string $userId = null
    ): Minutes {
        $body = array_filter([
            'sections' => $sections,
            'notes'    => $notes,
        ], fn($v) => $v !== null);

        $headers = $userId !== null ? ['X-User-Id' => $userId] : [];

        $data = $this->http->put("/v1/minutes/{$minutesId}", $body, $headers);
        return Minutes::fromArray($data);
    }

    /**
     * Get edit history for a minutes record.
     *
     * @return array<array<string, mixed>>
     */
    public function getVersions(string $minutesId): array
    {
        return $this->http->get("/v1/minutes/{$minutesId}/versions");
    }

    /** Delete minutes. */
    public function delete(string $minutesId): void
    {
        $this->http->delete("/v1/minutes/{$minutesId}");
    }
}
