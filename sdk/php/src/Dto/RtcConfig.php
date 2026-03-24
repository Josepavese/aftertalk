<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class RtcConfig
{
    /**
     * @readonly
     * @var list<array{urls: list<string>, username?: string, credential?: string}>
     */
    public array $iceServers;

    /**
     * @readonly
     * @var int
     */
    public int $ttl;

    /**
     * @readonly
     * @var string
     */
    public string $provider;

    /**
     * @param list<array{urls: list<string>, username?: string, credential?: string}> $iceServers
     */
    public function __construct(array $iceServers, int $ttl = 86400, string $provider = '')
    {
        $this->iceServers = $iceServers;
        $this->ttl        = $ttl;
        $this->provider   = $provider;
    }

    public static function fromArray(array $data): self
    {
        return new self(
            $data['ice_servers'] ?? [],
            $data['ttl']         ?? 86400,
            $data['provider']    ?? ''
        );
    }
}
