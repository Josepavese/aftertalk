<?php

declare(strict_types=1);

namespace Aftertalk\Tests\Unit;

use Aftertalk\Api\SessionsApi;
use Aftertalk\Config;
use Aftertalk\Dto\Session;
use Aftertalk\Exception\NotFoundException;
use Aftertalk\Http\HttpClient;
use PHPUnit\Framework\TestCase;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\ResponseInterface;
use Psr\Http\Message\StreamFactoryInterface;
use Psr\Http\Message\StreamInterface;

class SessionsApiTest extends TestCase
{
    // ─── helpers ────────────────────────────────────────────────────────────────

    private function makeSessionJson(array $overrides = []): string
    {
        return json_encode(array_merge([
            'session_id'        => 'sess-abc',
            'status'            => 'active',
            'participant_count' => 2,
            'participants'      => [
                ['participant_id' => 'p1', 'user_id' => 'doc', 'role' => 'terapeuta', 'token' => 'jwt1'],
                ['participant_id' => 'p2', 'user_id' => 'pat', 'role' => 'paziente',  'token' => 'jwt2'],
            ],
            'created_at' => '2026-03-19T10:00:00Z',
            'updated_at' => '2026-03-19T10:00:00Z',
            'template_id' => 'therapy',
            'stt_profile' => 'cloud',
            'llm_profile' => 'local',
        ], $overrides));
    }

    private function makeHttpClient(int $status, string $body): HttpClient
    {
        $cfg = new Config('http://localhost', 'key');

        $stream = $this->createMock(StreamInterface::class);
        $stream->method('__toString')->willReturn($body);
        $stream->method('getContents')->willReturn($body);

        $response = $this->createMock(ResponseInterface::class);
        $response->method('getStatusCode')->willReturn($status);
        $response->method('getBody')->willReturn($stream);

        $psrClient = $this->createMock(ClientInterface::class);
        $psrClient->method('sendRequest')->willReturn($response);

        $request = $this->createMock(\Psr\Http\Message\RequestInterface::class);
        $request->method('withHeader')->willReturnSelf();
        $request->method('withBody')->willReturnSelf();

        $requestFactory = $this->createMock(RequestFactoryInterface::class);
        $requestFactory->method('createRequest')->willReturn($request);

        $streamFactory = $this->createMock(StreamFactoryInterface::class);
        $streamFactory->method('createStream')->willReturn($stream);

        return new HttpClient($cfg, $psrClient, $requestFactory, $streamFactory);
    }

    // ─── create ─────────────────────────────────────────────────────────────────

    public function testCreateReturnsSession(): void
    {
        $http    = $this->makeHttpClient(201, $this->makeSessionJson());
        $api     = new SessionsApi($http);

        $session = $api->create(
            templateId:       'therapy',
            participantCount: 2,
            participants:     [
                ['user_id' => 'doc', 'role' => 'terapeuta'],
                ['user_id' => 'pat', 'role' => 'paziente'],
            ],
            sttProfile: 'cloud',
            llmProfile: 'local',
        );

        $this->assertInstanceOf(Session::class, $session);
        $this->assertSame('sess-abc', $session->id);
        $this->assertSame('active', $session->status);
        $this->assertSame('cloud', $session->sttProfile);
        $this->assertSame('local', $session->llmProfile);
        $this->assertCount(2, $session->participants);
    }

    // ─── get ────────────────────────────────────────────────────────────────────

    public function testGetReturnsSession(): void
    {
        $http    = $this->makeHttpClient(200, $this->makeSessionJson());
        $api     = new SessionsApi($http);
        $session = $api->get('sess-abc');

        $this->assertSame('sess-abc', $session->id);
        $this->assertSame('therapy', $session->templateId);
    }

    public function testGetThrowsNotFoundFor404(): void
    {
        $http = $this->makeHttpClient(404, '{"error":"session not found"}');
        $api  = new SessionsApi($http);

        $this->expectException(NotFoundException::class);
        $api->get('does-not-exist');
    }
}
