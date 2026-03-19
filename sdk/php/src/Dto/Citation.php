<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Citation
{
    public function __construct(
        public readonly string $text,
        public readonly string $role,
        public readonly int    $timestampMs,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            text:        $data['text'],
            role:        $data['role'],
            timestampMs: $data['timestamp_ms'],
        );
    }
}
