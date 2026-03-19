<?php

declare(strict_types=1);

namespace Aftertalk\Tests\Unit;

use Aftertalk\Exception\WebhookSignatureException;
use Aftertalk\Webhook\MinutesPayload;
use Aftertalk\Webhook\NotificationPayload;
use Aftertalk\Webhook\WebhookHandler;
use PHPUnit\Framework\TestCase;

class WebhookHandlerTest extends TestCase
{
    private const SECRET = 'test-secret-abc';

    private WebhookHandler $handler;

    protected function setUp(): void
    {
        $this->handler = new WebhookHandler(self::SECRET);
    }

    // ─── constructor ────────────────────────────────────────────────────────────

    public function testConstructorThrowsForEmptySecret(): void
    {
        $this->expectException(\LogicException::class);
        new \Aftertalk\Webhook\WebhookHandler('');
    }

    // ─── verifySignature ────────────────────────────────────────────────────────

    public function testVerifySignatureReturnsTrueForValidSignature(): void
    {
        $body      = '{"session_id":"abc"}';
        $signature = 'sha256=' . hash_hmac('sha256', $body, self::SECRET);

        $this->assertTrue($this->handler->verifySignature($body, $signature));
    }

    public function testVerifySignatureReturnsFalseForInvalidSignature(): void
    {
        $this->assertFalse($this->handler->verifySignature('{"session_id":"abc"}', 'sha256=bad'));
    }

    public function testVerifyThrowsOnInvalidSignature(): void
    {
        $this->expectException(WebhookSignatureException::class);
        $this->handler->verify('body', 'sha256=wrong');
    }

    // ─── parsePayload — push (minutes) ──────────────────────────────────────────

    public function testParseMinutesPayload(): void
    {
        $body = json_encode([
            'session_id' => 'sess-1',
            'timestamp'  => '2026-03-19T10:00:00Z',
            'minutes'    => [
                'id'           => 'min-1',
                'session_id'   => 'sess-1',
                'status'       => 'ready',
                'sections'     => ['themes' => ['stress']],
                'citations'    => [
                    ['text' => 'mi sento stanco', 'role' => 'paziente', 'timestamp_ms' => 3000],
                ],
                'version'      => 1,
                'generated_at' => '2026-03-19T10:05:00Z',
                'template_id'  => 'therapy',
                'provider'     => 'openai',
            ],
            'session_metadata' => '{"appointment_id":"appt_123"}',
            'participants'     => [
                ['user_id' => 'doc_456', 'role' => 'terapeuta'],
            ],
        ]);

        $payload = $this->handler->parsePayload($body);

        $this->assertInstanceOf(MinutesPayload::class, $payload);
        $this->assertSame('sess-1', $payload->sessionId);
        $this->assertSame('ready', $payload->minutes->status);
        $this->assertSame('mi sento stanco', $payload->minutes->citations[0]->text);
        $this->assertSame('appt_123', $payload->decodedMetadata()['appointment_id']);
        $this->assertCount(1, $payload->participants);
        $this->assertSame('terapeuta', $payload->participants[0]->role);
    }

    // ─── parsePayload — notify_pull ─────────────────────────────────────────────

    public function testParseNotificationPayload(): void
    {
        $body = json_encode([
            'session_id'   => 'sess-2',
            'timestamp'    => '2026-03-19T10:00:00Z',
            'retrieve_url' => 'https://example.com/minutes/pull/token123',
            'expires_at'   => '2026-03-19T11:00:00Z',
            'session_metadata' => null,
            'participants' => [],
        ]);

        $payload = $this->handler->parsePayload($body);

        $this->assertInstanceOf(NotificationPayload::class, $payload);
        $this->assertSame('sess-2', $payload->sessionId);
        $this->assertStringContainsString('token123', $payload->retrieveUrl);
        $this->assertNull($payload->decodedMetadata());
    }

    // ─── parsePayload — unknown shape ───────────────────────────────────────────

    public function testParseThrowsForUnknownShape(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->handler->parsePayload('{"session_id":"x","unknown_field":"y"}');
    }
}
