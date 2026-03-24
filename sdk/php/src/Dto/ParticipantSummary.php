<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

/** Compact participant record included in webhook payloads. */
class ParticipantSummary
{
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

    public function __construct(string $userId, string $role)
    {
        $this->userId = $userId;
        $this->role   = $role;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['user_id'],
            $data['role']
        );
    }
}
