# @provanity/worker-linux-x64

Linux x64 ProVanity headless worker binary for headless EVM and Tron vanity address generation.

For the interactive CLI, install `provanity` instead.

## Requirements

- Linux x64.
- NVIDIA GPU: Turing (`sm_75`) or newer. e.g. GTX 16-series, any RTX, T4, A100, H100, etc. Pascal and older (GTX 10-series, Titan X(p), Tesla M40/K80) are not supported — use [1inch/profanity2](https://github.com/1inch/profanity2) on those GPUs.
- NVIDIA Driver: >= 580.65 (CUDA 13.x baseline, Aug 2025+). Latest drivers are always recommended for best performance.

## Quickstart

Run the worker directly:

```bash
npx @provanity/worker-linux-x64
```

Run the worker:

```bash
npx @provanity/worker-linux-x64 \
  --pattern pattern:dead \
  --devices all \
  --progress-interval 1000 \
  --output result.json
```

The worker prints newline-delimited JSON events to stdout and exits when it
finds a match, receives a stop command, or encounters an error.

Run the same binary with `--help` for the full command reference.

## Usage

The examples below assume `provanity-worker` is on `PATH` or that you are
running the binary directly.

Generate an EVM vanity wallet:

```bash
provanity-worker --pattern leading:0:8
```

Generate a Tron vanity wallet:

```bash
provanity-worker --mode tron --pattern prefix:ABC   # or suffix:xyz
```

Run on specific CUDA devices:

```bash
provanity-worker --pattern pattern:deadXXXXbeef --devices 0,1
```

Run in split-key remote mode:

```bash
provanity-worker --pattern pattern:dead --init-pub <128 hex chars>
```

Enable encrypted WebSocket control:

```bash
provanity-worker \
  --pattern pattern:dead \
  --ws-server-port 8787 \
  --ws-key <64 hex chars> \
  --ws-progress-interval 1000
```

## Parameters

| Parameter | Default | Description |
|---|---:|---|
| `--mode` | `evm` | Wallet mode: `evm` or `tron`. |
| `--pattern` | | Required pattern for the selected mode. |
| `--devices` | `all` | Comma-separated CUDA device IDs such as `0,1`, or `all`. Interactive selection is not supported. |
| `-B, --batch-multiple` | `0` | Advanced CUDA batch size. Leave as `0` to use a known profile or autotune. |
| `--work-size` | `0` | CUDA threads per block. Leave as `0` to use a known profile or autotune. |
| `--progress-interval` | `0` | Milliseconds between stdout `progress` JSONL events. `0` disables stdout progress events. |
| `-o, --output` | | Write the final plaintext JSON result to this file. |
| `--init-pub` | | Split-key remote mode initial public key, as 128 hex characters. Result events omit `private_key`. |
| `--init-pub-stdin` | `false` | Read `init_pub` from one-line stdin JSON. Mutually exclusive with `--init-pub`. |
| `--ws-server-port` | `0` | Start encrypted WebSocket control on `0.0.0.0:N`. `0` disables WebSocket control. |
| `--ws-key` | | 32-byte WebSocket AES-GCM key, as 64 hex characters. Required when `--ws-server-port > 0`. |
| `--ws-key-stdin` | `false` | Read `ws_key` from one-line stdin JSON. Mutually exclusive with `--ws-key`. |
| `--ws-progress-interval` | `0` | Milliseconds between WebSocket `progress` broadcasts. `0` disables WebSocket progress events. |
| `--help` | | Print help. |
| `--version` | | Print version, OS/arch, config dir, and cache dir. |

When `--init-pub-stdin` or `--ws-key-stdin` is used, stdin must contain one JSON
line:

```json
{"init_pub":"<128 hex chars>","ws_key":"<64 hex chars>"}
```

Only the fields selected by `--init-pub-stdin` and `--ws-key-stdin` are
required. If both flags are set, put both values in the same first stdin line;
the worker reads that one line before the search starts.

## Split-Key Remote Mode

`--init-pub` lets a controller keep the initial private key off the worker host.
The worker receives only an initial public key and returns an `offset`. The
controller combines that offset with its initial private key to recover the
final private key.

All scalar values below are 32-byte big-endian secp256k1 values. The curve order
is:

```text
n = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141
```

Controller-side setup:

1. Generate an initial private key `k_init`, where `1 <= k_init < n`.
2. Compute the secp256k1 public point `P_init = k_init * G`.
3. Encode `init_pub` as `X || Y`, the 32-byte X coordinate followed by the
   32-byte Y coordinate, hex encoded as 128 characters. Do not include the
   uncompressed public-key prefix byte `04`.
4. Start the worker with `--init-pub <init_pub>`.

Worker-side search:

1. The worker searches offsets `d`.
2. For each candidate it derives `P_final = P_init + d * G`.
3. It derives the address from `P_final` and checks the selected pattern.
4. The final JSON result contains `offset` but omits `private_key`.

Controller-side finalization:

```text
k_final = (k_init + offset) mod n
```

Reject the result if `k_final == 0`, then derive the address from `k_final` and
verify that it equals the worker result. In EVM mode, the display address is
`0x` plus the last 20 bytes of Keccak-256 of the uncompressed public key without
the `04` prefix. In Tron mode, the same 20-byte address is prefixed with `0x41`
and encoded as Base58Check, producing the `T...` address.

## Pattern Syntax

EVM mode supports:

| Pattern | Description |
|---|---|
| `pattern:VALUE` | Match concrete hex nibbles from the start of the address. `X`, `x`, `*`, and `?` each match one nibble. |
| `leading:H:N` | Match at least `N` leading copies of one hex nibble `H`. |

Tron mode supports:

| Pattern | Description |
|---|---|
| `pattern:VALUE` | Match a Base58 address prefix. It must start with `T`; `*` and `?` are wildcards. |

Examples:

```bash
provanity-worker --pattern pattern:deadXXXXbeef
provanity-worker --pattern leading:0:8
provanity-worker --mode tron --pattern prefix:ABC   # or suffix:xyz
```

## Output

Stdout is JSON Lines. Common events are:

| Event | Description |
|---|---|
| `ready` | CUDA devices were detected. |
| `tuning` | CUDA parameter profile, autotune, or fallback state. |
| `progress` | Attempts, hashrate, elapsed time, and best score so far. |
| `candidate` | Best candidate improved or a match was found. |
| `result` | Final result. Includes `private_key` unless split-key remote mode is used. |
| `error` | Worker error message. |

## WebSocket Control

When `--ws-server-port` is set, the worker listens on `0.0.0.0:<port>` and
accepts encrypted WebSocket connections at `/`.

WebSocket control always requires a 32-byte encryption key, passed as
`--ws-key <64 hex chars>` or read from stdin with `--ws-key-stdin`. The worker
does not expose plaintext WebSocket control when started from the CLI.

### WebSocket Frames

All WebSocket messages are binary frames. The binary payload is:

```text
nonce || ciphertext || tag
```

| Field | Size | Description |
|---|---:|---|
| `nonce` | 12 bytes | AES-GCM nonce. Generate a fresh random nonce for every outgoing frame. |
| `ciphertext` | variable | AES-256-GCM encrypted UTF-8 JSON plaintext. |
| `tag` | 16 bytes | AES-GCM authentication tag. In Go this is appended by `gcm.Seal`. |

Encryption details:

- Algorithm: AES-256-GCM.
- Key: raw 32 bytes decoded from the 64 hex characters in `--ws-key`.
- Nonce: 12 bytes.
- Associated data: none.
- Plaintext: one JSON object encoded as UTF-8.
- On decrypt failure, the worker closes that WebSocket connection.

Node.js frame encoding example:

```js
import crypto from "node:crypto";

function encodeFrame(keyHex, value) {
  const key = Buffer.from(keyHex, "hex");
  const nonce = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv("aes-256-gcm", key, nonce);
  const plaintext = Buffer.from(JSON.stringify(value), "utf8");
  const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  return Buffer.concat([nonce, ciphertext, cipher.getAuthTag()]);
}

function decodeFrame(keyHex, frame) {
  const key = Buffer.from(keyHex, "hex");
  const nonce = frame.subarray(0, 12);
  const tag = frame.subarray(frame.length - 16);
  const ciphertext = frame.subarray(12, frame.length - 16);
  const decipher = crypto.createDecipheriv("aes-256-gcm", key, nonce);
  decipher.setAuthTag(tag);
  const plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
  return JSON.parse(plaintext.toString("utf8"));
}
```

### WebSocket Commands

Client-to-worker plaintext JSON before encryption:

| Command | Response |
|---|---|
| `{"command":"ping"}` | Worker sends a `pong` event. |
| `{"command":"stop"}` | Worker requests graceful cancellation. A `result` event is emitted if a best candidate is available. |

Invalid JSON produces an `error` event. Unknown commands produce an `error`
event with `unknown command: ...`.

### WebSocket Events

The server sends an encrypted `subscribed` event immediately after accepting a
client. It then broadcasts the same event shapes used by stdout JSON Lines:
`ready`, `tuning`, `progress`, `candidate`, `result`, and `error`.

`ready`, `tuning`, `candidate`, `result`, and `error` are broadcast whenever
they occur. `progress` broadcasts are controlled separately by
`--ws-progress-interval`; use `--progress-interval` only for stdout progress.

## Disclaimer

ProVanity generates crypto wallet private keys. Run the worker only on machines
and orchestration systems you trust with the key material or split-key search
inputs you provide.

Always verify that the generated private key derives to the expected address
before moving or receiving any funds.

## License

MIT
