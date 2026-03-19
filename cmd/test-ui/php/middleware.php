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
 *   php -S localhost:8081 middleware.php
 *   AFTERTALK_URL=http://localhost:8080 php -S localhost:8081 middleware.php
 */

declare(strict_types=1);

require_once __DIR__ . '/../../../sdk/php/vendor/autoload.php';

use Aftertalk\AftertalkClient;
use Aftertalk\Exception\NotFoundException;

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

$client = new AftertalkClient(baseUrl: $serverUrl, apiKey: $apiKey);
$path   = rtrim(parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH), '/') ?: '/';

// ─── Router ─────────────────────────────────────────────────────────────────

try {
    match ($_SERVER['REQUEST_METHOD'] . ' ' . $path) {
        'POST /rooms/join'   => roomsJoin($client),
        'POST /sessions/end' => sessionsEnd($client),
        'GET /minutes'       => minutes($client),
        default              => respond(404, ['error' => 'Not found']),
    };
} catch (\Throwable $e) {
    respond(500, ['error' => $e->getMessage()]);
}

// ─── Handlers ───────────────────────────────────────────────────────────────

function roomsJoin(AftertalkClient $client): void
{
    $body   = json_decode(file_get_contents('php://input'), true) ?? [];
    $result = $client->rooms->join(
        code:       $body['code']        ?? '',
        name:       $body['name']        ?? '',
        role:       $body['role']        ?? '',
        templateId: $body['template_id'] ?? null,
        sttProfile: $body['stt_profile'] ?? null,
        llmProfile: $body['llm_profile'] ?? null,
    );

    respond(200, ['session_id' => $result['sessionId'], 'token' => $result['token']]);
}

function sessionsEnd(AftertalkClient $client): void
{
    $body = json_decode(file_get_contents('php://input'), true) ?? [];
    $id   = $body['session_id'] ?? '';

    if (!$id) {
        respond(400, ['error' => 'session_id required']);
        return;
    }

    $client->sessions->end($id);
    http_response_code(204);
    echo '';
}

function minutes(AftertalkClient $client): void
{
    $sessionId = $_GET['session_id'] ?? '';

    if (!$sessionId) {
        respond(400, ['error' => 'session_id required']);
        return;
    }

    try {
        $minutes = $client->minutes->getBySession($sessionId);
        // json_encode serialises all public readonly properties (camelCase).
        // The TS frontend types are defined to match this shape.
        echo json_encode($minutes);
    } catch (NotFoundException) {
        respond(404, ['error' => 'Minutes not ready yet']);
    }
}

// ─── Helper ─────────────────────────────────────────────────────────────────

function respond(int $status, array $data): void
{
    http_response_code($status);
    echo json_encode($data);
}
