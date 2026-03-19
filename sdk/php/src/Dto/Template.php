<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Template
{
    /**
     * @param array<array{key:string,label:string}>                                 $roles
     * @param array<array{key:string,label:string,description:string,type:string}>  $sections
     */
    public function __construct(
        public readonly string $id,
        public readonly string $name,
        public readonly string $description,
        public readonly array  $roles,
        public readonly array  $sections,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            id:          $data['id'],
            name:        $data['name'],
            description: $data['description'] ?? '',
            roles:       $data['roles']       ?? [],
            sections:    $data['sections']    ?? [],
        );
    }
}
