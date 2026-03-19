<?php

declare(strict_types=1);

namespace Aftertalk\Exception;

class AftertalkException extends \RuntimeException
{
    public function __construct(
        string $message,
        private readonly int $statusCode = 0,
        private readonly ?array $body = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, $statusCode, $previous);
    }

    public function getStatusCode(): int
    {
        return $this->statusCode;
    }

    public function getBody(): ?array
    {
        return $this->body;
    }
}
