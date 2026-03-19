<?php

declare(strict_types=1);

namespace Aftertalk\Api;

use Aftertalk\Http\HttpClient;

class RoomsApi
{
    public function __construct(private readonly HttpClient $http) {}

    /**
     * Join or create a room session by code.
     *
     * Creates the session on the first call; subsequent participants with the same
     * code receive their own token for the same session.
     * Role is exclusive — two participants cannot share the same role in the same room.
     *
     * @return array{sessionId: string, token: string}
     */
    public function join(
        string  $code,
        string  $name,
        string  $role,
        ?string $templateId = null,
        ?string $sttProfile = null,
        ?string $llmProfile = null,
    ): array {
        $data = $this->http->post('/v1/rooms/join', array_filter([
            'code'        => $code,
            'name'        => $name,
            'role'        => $role,
            'template_id' => $templateId,
            'stt_profile' => $sttProfile,
            'llm_profile' => $llmProfile,
        ], fn($v) => $v !== null));

        return [
            'sessionId' => $data['session_id'],
            'token'     => $data['token'],
        ];
    }
}
