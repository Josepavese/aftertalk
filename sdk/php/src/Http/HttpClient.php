<?php

declare(strict_types=1);

namespace Aftertalk\Http;

use Aftertalk\Config;
use Aftertalk\Exception\AftertalkException;
use Aftertalk\Exception\AuthException;
use Aftertalk\Exception\NotFoundException;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\StreamFactoryInterface;

/**
 * Thin wrapper around any PSR-18 HTTP client.
 * Attaches the API-Key header, encodes/decodes JSON, and maps HTTP error
 * status codes to typed exceptions.
 */
class HttpClient
{
    public function __construct(
        private readonly Config $config,
        private readonly ClientInterface $client,
        private readonly RequestFactoryInterface $requestFactory,
        private readonly StreamFactoryInterface $streamFactory,
    ) {}

    /**
     * @param array<string, scalar|null> $query  Null values are filtered out automatically.
     * @return array<string, mixed>
     */
    public function get(string $path, array $query = []): array
    {
        $url = $this->url($path, $query);
        $request = $this->requestFactory
            ->createRequest('GET', $url)
            ->withHeader('X-API-Key', $this->config->apiKey)
            ->withHeader('Accept', 'application/json');

        return $this->send($request);
    }

    /** @return array<string, mixed> */
    public function post(string $path, array $body = []): array
    {
        $url = $this->url($path);
        $json = json_encode($body, JSON_THROW_ON_ERROR);
        $request = $this->requestFactory
            ->createRequest('POST', $url)
            ->withHeader('X-API-Key', $this->config->apiKey)
            ->withHeader('Content-Type', 'application/json')
            ->withHeader('Accept', 'application/json')
            ->withBody($this->streamFactory->createStream($json));

        return $this->send($request);
    }

    /** @return array<string, mixed> */
    public function put(string $path, array $body = [], array $headers = []): array
    {
        $url = $this->url($path);
        $json = json_encode($body, JSON_THROW_ON_ERROR);
        $request = $this->requestFactory
            ->createRequest('PUT', $url)
            ->withHeader('X-API-Key', $this->config->apiKey)
            ->withHeader('Content-Type', 'application/json')
            ->withHeader('Accept', 'application/json')
            ->withBody($this->streamFactory->createStream($json));

        foreach ($headers as $name => $value) {
            $request = $request->withHeader($name, $value);
        }

        return $this->send($request);
    }

    public function delete(string $path): void
    {
        $url = $this->url($path);
        $request = $this->requestFactory
            ->createRequest('DELETE', $url)
            ->withHeader('X-API-Key', $this->config->apiKey);

        $this->send($request);
    }

    // ─── private ────────────────────────────────────────────────────────────────

    private function url(string $path, array $query = []): string
    {
        $base = rtrim($this->config->baseUrl, '/');
        $url = $base . $path;
        if ($query !== []) {
            $url .= '?' . http_build_query(array_filter($query, fn($v) => $v !== null));
        }
        return $url;
    }

    private function send(\Psr\Http\Message\RequestInterface $request): array
    {
        try {
            $response = $this->client->sendRequest($request);
        } catch (\Psr\Http\Client\NetworkExceptionInterface $e) {
            throw new AftertalkException('Network unreachable: ' . $e->getMessage(), 0, null, $e);
        } catch (\Psr\Http\Client\RequestExceptionInterface $e) {
            throw new AftertalkException('Invalid request: ' . $e->getMessage(), 0, null, $e);
        } catch (\Psr\Http\Client\ClientExceptionInterface $e) {
            throw new AftertalkException('HTTP client error: ' . $e->getMessage(), 0, null, $e);
        }
        $status   = $response->getStatusCode();
        $raw      = (string) $response->getBody();
        $data     = $raw !== '' ? json_decode($raw, true, 512, JSON_THROW_ON_ERROR) : [];

        if ($status >= 200 && $status < 300) {
            // The server wraps successful responses in {"data": ...} — unwrap transparently.
            return $data['data'] ?? $data ?? [];
        }

        $message = $data['error'] ?? $data['message'] ?? "HTTP $status";

        match (true) {
            $status === 401 || $status === 403 => throw new AuthException($message, $status, $data),
            $status === 404                    => throw new NotFoundException($message, $status, $data),
            default                            => throw new AftertalkException($message, $status, $data),
        };
    }
}
