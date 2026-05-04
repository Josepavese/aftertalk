<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Minutes
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
    public string $sessionId;

    /**
     * @readonly
     * @var string
     */
    public string $status;

    /**
     * @readonly
     * @var array<string, mixed>  Conversation overview and chronological phases
     */
    public array $summary;

    /**
     * @readonly
     * @var array<string, mixed>  Keyed by section key (e.g. "themes", "next_steps")
     */
    public array $sections;

    /**
     * @readonly
     * @var Citation[]
     */
    public array $citations;

    /**
     * @readonly
     * @var int
     */
    public int $version;

    /**
     * @readonly
     * @var string
     */
    public string $generatedAt;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $templateId;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $provider;

    /**
     * @param array<string, mixed> $summary   Conversation overview and chronological phases
     * @param array<string, mixed> $sections  Keyed by section key (e.g. "themes", "next_steps")
     * @param Citation[]           $citations
     */
    public function __construct(
        string  $id,
        string  $sessionId,
        string  $status,
        array   $summary,
        array   $sections,
        array   $citations,
        int     $version,
        string  $generatedAt,
        ?string $templateId  = null,
        ?string $provider    = null
    ) {
        $this->id          = $id;
        $this->sessionId   = $sessionId;
        $this->status      = $status;
        $this->summary     = $summary;
        $this->sections    = $sections;
        $this->citations   = $citations;
        $this->version     = $version;
        $this->generatedAt = $generatedAt;
        $this->templateId  = $templateId;
        $this->provider    = $provider;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['id'],
            $data['session_id'],
            $data['status'],
            $data['summary']     ?? ['overview' => '', 'phases' => []],
            $data['sections']    ?? [],
            array_map(
                fn(array $c) => Citation::fromArray($c),
                $data['citations'] ?? []
            ),
            $data['version']     ?? 1,
            $data['generated_at'] ?? $data['created_at'] ?? '',
            $data['template_id'] ?? null,
            $data['provider']    ?? null
        );
    }
}
