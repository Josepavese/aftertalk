<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

/** Compact participant record included in webhook payloads. */
class ParticipantSummary
{
    public function __construct(
        public readonly string $userId,
        public readonly string $role,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            userId: $data['user_id'],
            role:   $data['role'],
        );
    }
}
