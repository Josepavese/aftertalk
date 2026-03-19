<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Participant
{
    public function __construct(
        public readonly string  $id,
        public readonly string  $userId,
        public readonly string  $role,
        public readonly string  $token,
        public readonly ?string $connectedAt  = null,
        public readonly ?string $audioStreamId = null,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            id:            $data['participant_id'],
            userId:        $data['user_id'],
            role:          $data['role'],
            token:         $data['token'],
            connectedAt:   $data['connected_at']   ?? null,
            audioStreamId: $data['audio_stream_id'] ?? null,
        );
    }
}
