<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Dto\Session;
use Aftertalk\Http\HttpClient;

class SessionsApi
{
    public function __construct(private readonly HttpClient $http) {}

    /**
     * Create a new session.
     *
     * @param string                                    $templateId       Template ID (e.g. "therapy")
     * @param int                                       $participantCount Total expected participants
     * @param array<array{user_id:string,role:string}>  $participants     Participant list
     * @param string|null                               $metadata         Opaque JSON string — never
     *                                                                    set from client input
     * @param string|null                               $sttProfile       STT provider profile
     *                                                                    (e.g. "local", "cloud")
     * @param string|null                               $llmProfile       LLM provider profile
     *                                                                    (e.g. "local", "cloud")
     */
    public function create(
        string  $templateId,
        int     $participantCount,
        array   $participants,
        ?string $metadata   = null,
        ?string $sttProfile = null,
        ?string $llmProfile = null,
    ): Session {
        $body = array_filter([
            'template_id'       => $templateId,
            'participant_count' => $participantCount,
            'participants'      => $participants,
            'metadata'          => $metadata,
            'stt_profile'       => $sttProfile,
            'llm_profile'       => $llmProfile,
        ], fn($v) => $v !== null);

        $data = $this->http->post('/v1/sessions', $body);
        return Session::fromArray($data);
    }

    /** Get a session by ID. */
    public function get(string $sessionId): Session
    {
        $data = $this->http->get("/v1/sessions/{$sessionId}");
        return Session::fromArray($data);
    }

    /**
     * List sessions with optional filters.
     *
     * @return Session[]
     */
    public function list(
        ?string $status = null,
        ?int    $limit  = null,
        ?int    $offset = null,
    ): array {
        $data = $this->http->get('/v1/sessions', [
            'status' => $status,
            'limit'  => $limit,
            'offset' => $offset,
        ]);

        return array_map(
            fn(array $s) => Session::fromArray($s),
            $data['items'] ?? $data,
        );
    }

    /**
     * End a session. Triggers transcription processing and minute generation.
     * This call is idempotent.
     */
    public function end(string $sessionId): void
    {
        $this->http->post("/v1/sessions/{$sessionId}/end");
    }

    /** Delete a session (must be in ended/completed/error status). */
    public function delete(string $sessionId): void
    {
        $this->http->delete("/v1/sessions/{$sessionId}");
    }
}
