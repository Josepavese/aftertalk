<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Session
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
    public string $status;

    /**
     * @readonly
     * @var int
     */
    public int $participantCount;

    /**
     * @readonly
     * @var Participant[]
     */
    public array $participants;

    /**
     * @readonly
     * @var string
     */
    public string $createdAt;

    /**
     * @readonly
     * @var string
     */
    public string $updatedAt;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $templateId;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $endedAt;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $metadata;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $sttProfile;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $llmProfile;

    /** @param Participant[] $participants */
    public function __construct(
        string  $id,
        string  $status,
        int     $participantCount,
        array   $participants,
        string  $createdAt,
        string  $updatedAt,
        ?string $templateId  = null,
        ?string $endedAt     = null,
        ?string $metadata    = null,
        ?string $sttProfile  = null,
        ?string $llmProfile  = null
    ) {
        $this->id               = $id;
        $this->status           = $status;
        $this->participantCount = $participantCount;
        $this->participants     = $participants;
        $this->createdAt        = $createdAt;
        $this->updatedAt        = $updatedAt;
        $this->templateId       = $templateId;
        $this->endedAt          = $endedAt;
        $this->metadata         = $metadata;
        $this->sttProfile       = $sttProfile;
        $this->llmProfile       = $llmProfile;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['session_id'] ?? $data['id'],
            $data['status'] ?? 'active',
            $data['participant_count'] ?? count($data['participants'] ?? []),
            array_map(
                fn(array $p) => Participant::fromArray($p),
                $data['participants'] ?? []
            ),
            $data['created_at'] ?? '',
            $data['updated_at'] ?? $data['created_at'] ?? '',
            $data['template_id']  ?? null,
            $data['ended_at']     ?? null,
            $data['metadata']     ?? null,
            $data['stt_profile']  ?? null,
            $data['llm_profile']  ?? null
        );
    }
}
