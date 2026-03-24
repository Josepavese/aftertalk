<?php

declare(strict_types=1);

namespace Aftertalk\Dto;

class ServerConfig
{
    /**
     * @readonly
     * @var Template[]
     */
    public array $templates;

    /**
     * @readonly
     * @var string
     */
    public string $defaultTemplateId;

    /**
     * @readonly
     * @var string[]
     */
    public array $sttProfiles;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $sttDefaultProfile;

    /**
     * @readonly
     * @var string[]
     */
    public array $llmProfiles;

    /**
     * @readonly
     * @var string|null
     */
    public ?string $llmDefaultProfile;

    /**
     * @param Template[] $templates
     * @param string[]   $sttProfiles
     * @param string[]   $llmProfiles
     */
    public function __construct(
        array   $templates,
        string  $defaultTemplateId,
        array   $sttProfiles       = [],
        ?string $sttDefaultProfile = null,
        array   $llmProfiles       = [],
        ?string $llmDefaultProfile = null
    ) {
        $this->templates          = $templates;
        $this->defaultTemplateId  = $defaultTemplateId;
        $this->sttProfiles        = $sttProfiles;
        $this->sttDefaultProfile  = $sttDefaultProfile;
        $this->llmProfiles        = $llmProfiles;
        $this->llmDefaultProfile  = $llmDefaultProfile;
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            array_map(fn(array $t) => Template::fromArray($t), $data['templates'] ?? []),
            $data['default_template_id'] ?? '',
            $data['stt_profiles']        ?? [],
            $data['default_stt_profile'] ?? null,
            $data['llm_profiles']        ?? [],
            $data['default_llm_profile'] ?? null
        );
    }
}
