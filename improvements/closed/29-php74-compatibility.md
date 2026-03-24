# Improvement #29 — PHP 7.4 Compatibility for PHP SDK

## Status: CLOSED

## Problem

The PHP SDK uses PHP 8.0/8.1+ syntax exclusively, making it incompatible with
production servers still on PHP 7.4 (EOL but still widely deployed — e.g.
Ubuntu 20.04 default, many shared hosting providers, the MondoPsicologi server
itself runs PHP 7.4.33).

## Breaking syntax inventory

| Feature | PHP version | Files affected |
|---------|-------------|----------------|
| Constructor property promotion (`public function __construct(private X $y)`) | 8.0+ | All Api/, Dto/, Http/, Webhook/, Config.php, Exception/AftertalkException.php |
| `readonly` properties | 8.1+ | All Dto/, AftertalkClient.php, Config.php, WebhookHandler.php |
| Named arguments (`new Foo(key: $val)`) | 8.0+ | Dto/Minutes.php, Dto/Session.php, Dto/Participant.php, Dto/ParticipantSummary.php, Dto/Citation.php, Dto/Template.php, Dto/RtcConfig.php, Dto/ServerConfig.php, Webhook/MinutesPayload.php, Webhook/NotificationPayload.php, AftertalkClient.php |
| Union return types (`A\|B`) | 8.0+ | Webhook/WebhookHandler.php |
| `catch (Exception)` without variable | 8.0+ | Dto/Minutes.php (via MinutesPayload), Webhook/NotificationPayload.php |
| `match` expression | 8.0+ | Http/HttpClient.php |

## Strategy

**Minimum**: PHP 7.4 (uses typed properties, arrow functions, null coalescing,
`JSON_THROW_ON_ERROR` — all PHP 7.4+).

**No runtime version checks**: the codebase is a single implementation
compatible with all PHP >= 7.4. It does not branch on `PHP_MAJOR_VERSION`.

**PHP 8.x leverage**: the same code runs on PHP 8.x without any changes
(syntax is forward-compatible). PHP 8.x users benefit from:
- Better JIT performance
- Stricter type coercions under `declare(strict_types=1)` (already set)
- Improved error messages
- Static analysis with `@readonly` PHPDoc annotations (Psalm/PHPStan understand these)

**Public API surface preserved**: `$client->sessions->create(...)` stays as-is.
Public properties remain `public` (no `readonly` keyword) but are annotated
`/** @readonly */` so static analyzers enforce immutability on PHP 8.x.

## Changes per file

### composer.json
- `"php": ">=8.1"` → `"php": ">=7.4"`
- Add `"suggest"` entry: `"php": "Use PHP 8.1+ for readonly property enforcement and JIT"`

### Config.php
- Remove constructor property promotion + `readonly`
- Use typed properties PHP 7.4 style: `private string $baseUrl;`
- Keep `__construct` with explicit `$this->x = $x` assignments

### Http/HttpClient.php
- Remove constructor property promotion + `readonly`
- Replace `match (true)` with `if/elseif`

### AftertalkClient.php
- Remove `readonly` from public properties (`public SessionsApi $sessions;`)
- Add `/** @readonly */` PHPDoc to each
- Replace named args in `new Config(baseUrl: $x, ...)` with positional

### Api/ConfigApi.php, MinutesApi.php, RoomsApi.php, SessionsApi.php, TranscriptionsApi.php
- Replace `public function __construct(private readonly HttpClient $http) {}`
  with explicit `private HttpClient $http;` + constructor assignment

### Dto/* (all 8 DTO files)
- Remove constructor property promotion + `readonly`
- Use typed properties: `public string $id;` with `/** @readonly */`
- Replace named arguments with positional in `new self(...)` factory calls
- Fix `catch (\JsonException)` → `catch (\JsonException $e)`

### Webhook/MinutesPayload.php, NotificationPayload.php
- Same as Dto: property promotion → explicit, named args → positional
- Fix `catch (\JsonException)` → `catch (\JsonException $e)`

### Webhook/WebhookHandler.php
- Remove constructor property promotion + `readonly`
- Replace union return type `MinutesPayload|NotificationPayload` with no type
  hint + PHPDoc `@return MinutesPayload|NotificationPayload`

### Exception/AftertalkException.php
- Remove constructor property promotion + `readonly`
- Use parent `Exception` constructor + explicit assignments

## Outcome
- `composer.json` minimum: `>=7.4`
- Runs on PHP 7.4, 8.0, 8.1, 8.2, 8.3 without changes
- Public API unchanged
- Static analyzers (Psalm, PHPStan) still enforce `@readonly` on PHP 8.x
