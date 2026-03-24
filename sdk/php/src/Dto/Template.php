<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class Template
{
    /**
     * @readonly
     * @var string
     */
    public string $id;

    /**
     * @readonly
     * @var string
     */
    public string $name;

    /**
     * @readonly
     * @var string
     */
    public string $description;

    /**
     * @readonly
     * @var array<array{key:string,label:string}>
     */
    public array $roles;

    /**
     * @readonly
     * @var array<array{key:string,label:string,description:string,type:string}>
     */
    public array $sections;

    /**
     * @param array<array{key:string,label:string}>                                $roles
     * @param array<array{key:string,label:string,description:string,type:string}> $sections
     */
    public function __construct(
        string $id,
        string $name,
        string $description,
        array  $roles,
        array  $sections
    ) {
        $this->id          = $id;
        $this->name        = $name;
        $this->description = $description;
        $this->roles       = $roles;
        $this->sections    = $sections;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            $data['id'],
            $data['name'],
            $data['description'] ?? '',
            $data['roles']       ?? [],
            $data['sections']    ?? []
        );
    }
}
