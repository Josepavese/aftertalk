<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Citation
{
    /**
     * @readonly
     * @var string
     */
    public string $text;

    /**
     * @readonly
     * @var string
     */
    public string $role;

    /**
     * @readonly
     * @var int
     */
    public int $timestampMs;

    public function __construct(string $text, string $role, int $timestampMs)
    {
        $this->text        = $text;
        $this->role        = $role;
        $this->timestampMs = $timestampMs;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['text'],
            $data['role'],
            $data['timestamp_ms']
        );
    }
}
