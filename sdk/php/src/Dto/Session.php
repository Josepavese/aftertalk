<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Session
{
    /** @param Participant[] $participants */
    public function __construct(
        public readonly string  $id,
        public readonly string  $status,
        public readonly int     $participantCount,
        public readonly array   $participants,
        public readonly string  $createdAt,
        public readonly string  $updatedAt,
        public readonly ?string $templateId  = null,
        public readonly ?string $endedAt     = null,
        public readonly ?string $metadata    = null,
        public readonly ?string $sttProfile  = null,
        public readonly ?string $llmProfile  = null,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            id:               $data['session_id'],
            status:           $data['status'],
            participantCount: $data['participant_count'] ?? 0,
            participants:     array_map(
                fn(array $p) => Participant::fromArray($p),
                $data['participants'] ?? [],
            ),
            createdAt:        $data['created_at'],
            updatedAt:        $data['updated_at'] ?? $data['created_at'],
            templateId:       $data['template_id']  ?? null,
            endedAt:          $data['ended_at']     ?? null,
            metadata:         $data['metadata']     ?? null,
            sttProfile:       $data['stt_profile']  ?? null,
            llmProfile:       $data['llm_profile']  ?? null,
        );
    }
}
