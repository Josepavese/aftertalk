<?php

/**
 * Aftertalk PHP Middleware — test-ui reference implementation
 *
 * This file is the server-side layer between the browser frontend and the
 * Aftertalk server. It holds the API key and exposes a minimal REST API that
 * the browser can call without ever knowing the API key.
 *
 * SECURITY:
 *   In production, store the API key in AFTERTALK_API_KEY env var and remove
 *   the X-API-Key header fallback. The key must never leave this server.
 *   The X-API-Key fallback exists here ONLY for local test convenience.
 *
 * Usage (local dev):
 *   php -S localhost:8081 cmd/test-ui/php/middleware.php
 *   AFTERTALK_URL=http://localhost:8080 php -S localhost:8081 cmd/test-ui/php/middleware.php
 *
 * Usage (production server on port 8082, reverse-proxied via Apache):
 *   AFTERTALK_URL=http://127.0.0.1:9080 AFTERTALK_API_KEY=<key> php -S 127.0.0.1:8082 middleware.php
 *
 * NOTE: This file is intentionally self-contained — no Composer or SDK required.
 *       Raw curl is used so it runs on any PHP 7.4+ install out of the box.
 */

declare(strict_types=1);

// ─── CORS (dev: allow all origins) ──────────────────────────────────────────

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: GET, POST, OPTIONS');
header('Access-Control-Allow-Headers: Content-Type, X-API-Key');
header('Content-Type: application/json');

if ($_SERVER['REQUEST_METHOD'] === 'OPTIONS') {
    http_response_code(204);
    exit;
}

// ─── Config ─────────────────────────────────────────────────────────────────

// PRODUCTION: set AFTERTALK_API_KEY env var and remove the header fallback.
// TEST: the browser forwards its local API key via X-API-Key for convenience.
$apiKey    = getenv('AFTERTALK_API_KEY') ?: ($_SERVER['HTTP_X_API_KEY'] ?? '');
$serverUrl = rtrim(getenv('AFTERTALK_URL') ?: 'http://localhost:8080', '/');

if (!$apiKey) {
    respond(401, ['error' => 'API key required. Set AFTERTALK_API_KEY env var.']);
    exit;
}

$method = $_SERVER['REQUEST_METHOD'];
$path   = rtrim(parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH), '/') ?: '/';

// ─── Router ─────────────────────────────────────────────────────────────────

try {
    $route = $method . ' ' . $path;
    if ($route === 'POST /rooms/join') {
        roomsJoin($serverUrl, $apiKey);
    } elseif ($route === 'POST /sessions/end') {
        sessionsEnd($serverUrl, $apiKey);
    } elseif ($route === 'GET /minutes') {
        minutes($serverUrl, $apiKey);
    } else {
        respond(404, ['error' => 'Not found']);
    }
} catch (\Throwable $e) {
    respond(500, ['error' => $e->getMessage()]);
}

// ─── Handlers ───────────────────────────────────────────────────────────────

function roomsJoin(string $serverUrl, string $apiKey): void
{
    $body = json_decode(file_get_contents('php://input'), true) ?: [];
    $payload = array_filter([
        'code'        => $body['code']        ?? '',
        'name'        => $body['name']        ?? '',
        'role'        => $body['role']        ?? '',
        'template_id' => $body['template_id'] ?? null,
        'stt_profile' => $body['stt_profile'] ?? null,
        'llm_profile' => $body['llm_profile'] ?? null,
    ], function ($v) { return $v !== null && $v !== ''; });

    // Go wraps response in {"data": {...}}
    $data = goPost($serverUrl . '/v1/rooms/join', $apiKey, $payload);
    respond(200, ['session_id' => $data['session_id'], 'token' => $data['token']]);
}

function sessionsEnd(string $serverUrl, string $apiKey): void
{
    $body = json_decode(file_get_contents('php://input'), true) ?: [];
    $id   = $body['session_id'] ?? '';

    if (!$id) {
        respond(400, ['error' => 'session_id required']);
        return;
    }

    goPost($serverUrl . '/v1/sessions/' . urlencode($id) . '/end', $apiKey, []);
    http_response_code(204);
    echo '';
}

function minutes(string $serverUrl, string $apiKey): void
{
    $sessionId = $_GET['session_id'] ?? '';

    if (!$sessionId) {
        respond(400, ['error' => 'session_id required']);
        return;
    }

    // Go uses render.JSON here (no data envelope) — returns snake_case struct.
    $data = goGet($serverUrl . '/v1/minutes?session_id=' . urlencode($sessionId), $apiKey);

    // Transform snake_case → camelCase so the TS frontend interface stays clean.
    $citations = array_map(function ($c) {
        return [
            'text'        => $c['text'],
            'role'        => $c['role'],
            'timestampMs' => $c['timestamp_ms'] ?? 0,
        ];
    }, $data['citations'] ?? []);

    respond(200, [
        'status'      => $data['status'],
        'sections'    => $data['sections'] ?? (object) [],
        'citations'   => $citations,
        'templateId'  => $data['template_id'] ?? '',
        'version'     => $data['version']     ?? 1,
        'generatedAt' => $data['generated_at'] ?? '',
        'provider'    => $data['provider']    ?? '',
    ]);
}

// ─── HTTP helpers (raw curl, no SDK) ────────────────────────────────────────

/** POST JSON to the Aftertalk server. Returns unwrapped data (strips {"data":...} envelope). */
function goPost(string $url, string $apiKey, array $body): array
{
    $ch = curl_init($url);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_POST           => true,
        CURLOPT_POSTFIELDS     => json_encode($body),
        CURLOPT_HTTPHEADER     => [
            'Content-Type: application/json',
            'Accept: application/json',
            'Authorization: Bearer ' . $apiKey,
        ],
        CURLOPT_TIMEOUT        => 30,
    ]);
    $raw    = curl_exec($ch);
    $status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);

    if ($status === 204) {
        return [];
    }

    $decoded = json_decode($raw, true) ?: [];
    if ($status >= 400) {
        $msg = $decoded['error'] ?? $decoded['message'] ?? "HTTP $status";
        throw new \RuntimeException($msg, $status);
    }

    // Unwrap {"data": {...}} envelope used by Go success responses.
    return $decoded['data'] ?? $decoded;
}

/** GET from the Aftertalk server. Returns raw decoded JSON (no envelope unwrap for render.JSON endpoints). */
function goGet(string $url, string $apiKey): array
{
    $ch = curl_init($url);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HTTPHEADER     => [
            'Accept: application/json',
            'Authorization: Bearer ' . $apiKey,
        ],
        CURLOPT_TIMEOUT        => 30,
    ]);
    $raw    = curl_exec($ch);
    $status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);

    $decoded = json_decode($raw, true) ?: [];
    if ($status >= 400) {
        $msg = $decoded['error'] ?? $decoded['message'] ?? "HTTP $status";
        throw new \RuntimeException($msg, $status);
    }

    // Some endpoints use response.OK envelope, some use render.JSON — handle both.
    return $decoded['data'] ?? $decoded;
}

// ─── Helper ─────────────────────────────────────────────────────────────────

function respond(int $status, array $data): void
{
    http_response_code($status);
    echo json_encode($data);
}
