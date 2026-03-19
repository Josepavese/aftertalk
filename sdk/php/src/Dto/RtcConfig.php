<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class RtcConfig
{
    /**
     * @param list<array{urls: list<string>, username?: string, credential?: string}> $iceServers
     */
    public function __construct(
        public readonly array  $iceServers,
        public readonly int    $ttl      = 86400,
        public readonly string $provider = '',
    ) {}

    public static function fromArray(array $data): self
    {
        return new self(
            iceServers: $data['ice_servers'] ?? [],
            ttl:        $data['ttl']         ?? 86400,
            provider:   $data['provider']    ?? '',
        );
    }
}
