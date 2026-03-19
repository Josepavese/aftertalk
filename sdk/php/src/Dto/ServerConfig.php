<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class ServerConfig
{
    /**
     * @param Template[] $templates
     * @param string[]   $sttProfiles
     * @param string[]   $llmProfiles
     */
    public function __construct(
        public readonly array   $templates,
        public readonly string  $defaultTemplateId,
        public readonly array   $sttProfiles       = [],
        public readonly ?string $sttDefaultProfile = null,
        public readonly array   $llmProfiles       = [],
        public readonly ?string $llmDefaultProfile = null,
    ) {}

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            templates:          array_map(fn(array $t) => Template::fromArray($t), $data['templates'] ?? []),
            defaultTemplateId:  $data['default_template_id'] ?? '',
            sttProfiles:        $data['stt_profiles']        ?? [],
            sttDefaultProfile:  $data['default_stt_profile'] ?? null,
            llmProfiles:        $data['llm_profiles']        ?? [],
            llmDefaultProfile:  $data['default_llm_profile'] ?? null,
        );
    }
}
