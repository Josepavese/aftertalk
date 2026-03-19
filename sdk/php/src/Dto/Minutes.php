<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Minutes
{
    /**
     * @param array<string, mixed>  $sections  Keyed by section key (e.g. "themes", "next_steps")
     * @param Citation[]            $citations
     */
    public function __construct(
        public readonly string  $id,
        public readonly string  $sessionId,
        public readonly string  $status,
        public readonly array   $sections,
        public readonly array   $citations,
        public readonly int     $version,
        public readonly string  $generatedAt,
        public readonly ?string $templateId  = null,
        public readonly ?string $provider    = null,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            id:          $data['id'],
            sessionId:   $data['session_id'],
            status:      $data['status'],
            sections:    $data['sections']    ?? [],
            citations:   array_map(
                fn(array $c) => Citation::fromArray($c),
                $data['citations'] ?? [],
            ),
            version:     $data['version']     ?? 1,
            generatedAt: $data['generated_at'] ?? $data['created_at'] ?? '',
            templateId:  $data['template_id'] ?? null,
            provider:    $data['provider']    ?? null,
        );
    }
}
