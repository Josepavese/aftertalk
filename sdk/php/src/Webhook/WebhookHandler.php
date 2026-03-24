<?php

declare(strict_types=1);

namespace Aftertalk\Webhook;

use Aftertalk\Exception\WebhookSignatureException;

/**
 * Verifies HMAC-SHA256 signatures and parses webhook payloads.
 *
 * Aftertalk signs every webhook POST body with HMAC-SHA256 using the
 * `webhook_secret` configured on the server. The signature is sent in the
 * `X-Aftertalk-Signature` request header as `sha256=<hex>`.
 */
class WebhookHandler
{
    /**
     * @readonly
     * @var string
     */
    private string $secret;

    public function __construct(string $secret)
    {
        if ($secret === '') {
            throw new \LogicException(
                'WebhookHandler requires a non-empty secret. Set webhookSecret in AftertalkClient constructor.'
            );
        }
        $this->secret = $secret;
    }

    /**
     * Verify the webhook signature.
     *
     * @param string $body            Raw request body (do NOT json_decode before passing)
     * @param string $signatureHeader Value of the `X-Aftertalk-Signature` header
     *
     * @throws WebhookSignatureException when the signature is invalid
     */
    public function verify(string $body, string $signatureHeader): void
    {
        $expected = 'sha256=' . hash_hmac('sha256', $body, $this->secret);
        if (!hash_equals($expected, $signatureHeader)) {
            throw new WebhookSignatureException();
        }
    }

    /**
     * Returns true when the signature is valid, false otherwise.
     * Use `verify()` instead when you want an exception on failure.
     */
    public function verifySignature(string $body, string $signatureHeader): bool
    {
        try {
            $this->verify($body, $signatureHeader);
            return true;
        } catch (WebhookSignatureException $e) {
            return false;
        }
    }

    /**
     * Parse a verified webhook payload.
     *
     * Automatically detects the payload type:
     * - If it contains a `minutes` key → MinutesPayload (push mode)
     * - If it contains a `retrieve_url` key → NotificationPayload (notify_pull mode)
     *
     * Call `verify()` or `verifySignature()` before parsing.
     *
     * @return MinutesPayload|NotificationPayload
     * @throws \InvalidArgumentException for unknown payload shapes
     */
    public function parsePayload(string $body)
    {
        $data = json_decode($body, true, 512, JSON_THROW_ON_ERROR);

        if (isset($data['minutes'])) {
            return MinutesPayload::fromArray($data);
        }
        if (isset($data['retrieve_url'])) {
            return NotificationPayload::fromArray($data);
        }

        throw new \InvalidArgumentException(
            'Unknown webhook payload shape — expected "minutes" or "retrieve_url" key'
        );
    }
}
