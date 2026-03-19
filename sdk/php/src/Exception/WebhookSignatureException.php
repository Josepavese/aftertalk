<?php

declare(strict_types=1);

namespace Aftertalk\Exception;

/** Thrown when an incoming webhook's HMAC signature does not match. */
class WebhookSignatureException extends AftertalkException
{
    public function __construct(string $message = 'Webhook signature verification failed')
    {
        parent::__construct($message, 401);
    }
}
