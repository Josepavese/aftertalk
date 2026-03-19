<?php

declare(strict_types=1);

namespace Aftertalk;

use Aftertalk\Api\ConfigApi;
use Aftertalk\Api\MinutesApi;
use Aftertalk\Api\RoomsApi;
use Aftertalk\Api\SessionsApi;
use Aftertalk\Api\TranscriptionsApi;
use Aftertalk\Http\HttpClient;
use Aftertalk\Webhook\WebhookHandler;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\StreamFactoryInterface;

/**
 * Main entry point for the Aftertalk PHP SDK.
 *
 * Usage:
 * ```php
 * $client = new AftertalkClient(
 *     baseUrl:       'https://aftertalk.yourserver.com',
 *     apiKey:        $_ENV['AFTERTALK_API_KEY'],
 *     webhookSecret: $_ENV['AFTERTALK_WEBHOOK_SECRET'],
 * );
 *
 * $session = $client->sessions->create(
 *     templateId:       'therapy',
 *     participantCount: 2,
 *     participants:     [
 *         ['user_id' => 'doc_456', 'role' => 'terapeuta'],
 *         ['user_id' => 'pat_789', 'role' => 'paziente'],
 *     ],
 *     sttProfile: 'cloud',
 *     llmProfile: 'cloud',
 * );
 * ```
 *
 * PSR-18 client injection:
 * By default the SDK auto-discovers a PSR-18 client via `php-http/discovery`.
 * To inject a specific client (e.g. Guzzle, Symfony):
 * ```php
 * $client = new AftertalkClient(
 *     ...,
 *     httpClient:     new \GuzzleHttp\Client(),
 *     requestFactory: new \GuzzleHttp\Psr7\HttpFactory(),
 *     streamFactory:  new \GuzzleHttp\Psr7\HttpFactory(),
 * );
 * ```
 */
class AftertalkClient
{
    public readonly SessionsApi      $sessions;
    public readonly MinutesApi       $minutes;
    public readonly TranscriptionsApi $transcriptions;
    public readonly ConfigApi        $config;
    public readonly RoomsApi         $rooms;
    public readonly ?WebhookHandler  $webhook;

    public function __construct(
        string                   $baseUrl,
        string                   $apiKey,
        ?string                  $webhookSecret  = null,
        int                      $timeout        = 30,
        ?ClientInterface         $httpClient     = null,
        ?RequestFactoryInterface $requestFactory = null,
        ?StreamFactoryInterface  $streamFactory  = null,
    ) {
        $cfg = new Config(
            baseUrl:       $baseUrl,
            apiKey:        $apiKey,
            webhookSecret: $webhookSecret,
            timeout:       $timeout,
        );

        [$httpClient, $requestFactory, $streamFactory] =
            $this->resolveHttpDeps($httpClient, $requestFactory, $streamFactory);

        $http = new HttpClient($cfg, $httpClient, $requestFactory, $streamFactory);

        $this->sessions       = new SessionsApi($http);
        $this->minutes        = new MinutesApi($http);
        $this->transcriptions = new TranscriptionsApi($http);
        $this->config         = new ConfigApi($http);
        $this->rooms          = new RoomsApi($http);
        $this->webhook        = $webhookSecret !== null ? new WebhookHandler($webhookSecret) : null;
    }

    // ─── private ────────────────────────────────────────────────────────────────

    private function resolveHttpDeps(
        ?ClientInterface         $httpClient,
        ?RequestFactoryInterface $requestFactory,
        ?StreamFactoryInterface  $streamFactory,
    ): array {
        $injected = array_filter([$httpClient, $requestFactory, $streamFactory], fn($v) => $v !== null);
        $count = count($injected);
        if ($count > 0 && $count < 3) {
            throw new \InvalidArgumentException(
                'When injecting HTTP dependencies, all three must be provided: $httpClient, $requestFactory, $streamFactory.'
            );
        }

        // If all three are provided, use them directly.
        if ($httpClient !== null && $requestFactory !== null && $streamFactory !== null) {
            return [$httpClient, $requestFactory, $streamFactory];
        }

        // Auto-discover via php-http/discovery (if available) or Guzzle.
        if (class_exists(\Http\Discovery\Psr18ClientDiscovery::class)) {
            return [
                $httpClient     ?? \Http\Discovery\Psr18ClientDiscovery::find(),
                $requestFactory ?? \Http\Discovery\Psr17FactoryDiscovery::findRequestFactory(),
                $streamFactory  ?? \Http\Discovery\Psr17FactoryDiscovery::findStreamFactory(),
            ];
        }

        // Fallback to Guzzle if directly available.
        if (class_exists(\GuzzleHttp\Client::class)) {
            $factory = new \GuzzleHttp\Psr7\HttpFactory();
            return [
                $httpClient     ?? new \GuzzleHttp\Client(),
                $requestFactory ?? $factory,
                $streamFactory  ?? $factory,
            ];
        }

        throw new \RuntimeException(
            'No PSR-18 HTTP client found. Install guzzlehttp/guzzle or any PSR-18 client, ' .
            'or pass $httpClient/$requestFactory/$streamFactory explicitly.'
        );
    }
}
