<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Participant
{
    /**
     * @readonly
     * @var string
     */
    public string $id;

    /**
     * @readonly
     * @var string
     */
    public string $userId;

    /**
     * @readonly
     * @var string
     */
    public string $role;

    /**
     * @readonly
     * @var string
     */
    public string $token;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $connectedAt;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $audioStreamId;

    public function __construct(
        string  $id,
        string  $userId,
        string  $role,
        string  $token,
        ?string $connectedAt   = null,
        ?string $audioStreamId = null
    ) {
        $this->id            = $id;
        $this->userId        = $userId;
        $this->role          = $role;
        $this->token         = $token;
        $this->connectedAt   = $connectedAt;
        $this->audioStreamId = $audioStreamId;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['participant_id'] ?? $data['id'],
            $data['user_id'],
            $data['role'],
            $data['token'] ?? '',
            $data['connected_at']   ?? null,
            $data['audio_stream_id'] ?? null
        );
    }
}
