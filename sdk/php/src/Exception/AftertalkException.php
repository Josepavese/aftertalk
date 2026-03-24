<?php

declare(strict_types=1);

namespace Aftertalk\Exception;

class AftertalkException extends \RuntimeException
{
    /**
     * @readonly
     * @var int
     */
    private int $statusCode;

    /**
     * @readonly
     * @var array<mixed>|null
     */
    private ?array $body;

    public function __construct(
        string     $message,
        int        $statusCode = 0,
        ?array     $body       = null,
        ?\Throwable $previous  = null
    ) {
        $this->statusCode = $statusCode;
        $this->body       = $body;
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
