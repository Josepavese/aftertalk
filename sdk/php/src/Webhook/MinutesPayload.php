<?php

declare(strict_types=1);

namespace Aftertalk\Webhook;

use Aftertalk\Dto\Minutes;
use Aftertalk\Dto\ParticipantSummary;

/**
 * Payload for "push" mode webhooks.
 * The full minutes JSON is delivered directly in the POST body.
 */
class MinutesPayload
{
    /**
     * @readonly
     * @var string
     */
    public string $sessionId;

    /**
     * @readonly
     * @var string
     */
    public string $timestamp;

    /**
     * @readonly
     * @var Minutes
     */
    public Minutes $minutes;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $sessionMetadata;

    /**
     * @readonly
     * @var ParticipantSummary[]
     */
    public array $participants;

    /**
     * @param ParticipantSummary[] $participants
     */
    public function __construct(
        string  $sessionId,
        string  $timestamp,
        Minutes $minutes,
        ?string $sessionMetadata = null,
        array   $participants    = []
    ) {
        $this->sessionId       = $sessionId;
        $this->timestamp       = $timestamp;
        $this->minutes         = $minutes;
        $this->sessionMetadata = $sessionMetadata;
        $this->participants    = $participants;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['session_id'],
            $data['timestamp'],
            Minutes::fromArray($data['minutes']),
            $data['session_metadata'] ?? null,
            array_map(
                fn(array $p) => ParticipantSummary::fromArray($p),
                $data['participants'] ?? []
            )
        );
    }

    /**
     * Decode session_metadata JSON. Returns null if not set or invalid JSON.
     *
     * @return array<string, mixed>|null
     */
    public function decodedMetadata(): ?array
    {
        if ($this->sessionMetadata === null) {
            return null;
        }
        try {
            return json_decode($this->sessionMetadata, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            return null;
        }
    }
}
