<?php

declare(strict_types=1);

namespace Aftertalk\Webhook;

use Aftertalk\Dto\ParticipantSummary;

/**
 * Payload for "notify_pull" mode webhooks.
 * Contains only a signed, single-use retrieval URL — no clinical data.
 */
class NotificationPayload
{
    /**
     * @param ParticipantSummary[] $participants
     */
    public function __construct(
        public readonly string  $sessionId,
        public readonly string  $timestamp,
        public readonly string  $retrieveUrl,
        public readonly string  $expiresAt,
        public readonly ?string $sessionMetadata = null,
        public readonly array   $participants    = [],
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            sessionId:       $data['session_id'],
            timestamp:       $data['timestamp'],
            retrieveUrl:     $data['retrieve_url'],
            expiresAt:       $data['expires_at'],
            sessionMetadata: $data['session_metadata'] ?? null,
            participants:    array_map(
                fn(array $p) => ParticipantSummary::fromArray($p),
                $data['participants'] ?? [],
            ),
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
        } catch (\JsonException) {
            return null;
        }
    }
}
