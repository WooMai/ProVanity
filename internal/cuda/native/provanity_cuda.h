#ifndef PROVANITY_CUDA_BACKEND_H
#define PROVANITY_CUDA_BACKEND_H

#include <stdint.h>

#ifdef __cplusplus
#define PROVANITY_CUDA_EXTERN extern "C"
#else
#define PROVANITY_CUDA_EXTERN extern
#endif

#ifdef _WIN32
#define PROVANITY_CUDA_API PROVANITY_CUDA_EXTERN __declspec(dllexport)
#define PROVANITY_CUDA_CALL __stdcall
#else
#define PROVANITY_CUDA_API PROVANITY_CUDA_EXTERN __attribute__((visibility("default")))
#define PROVANITY_CUDA_CALL
#endif

#define PROVANITY_CUDA_MAX_DEVICES 16
#define PROVANITY_CUDA_MAX_SCORE 40
#define PROVANITY_CUDA_PATTERN_LEN 40
#define PROVANITY_CUDA_PATTERN_WILDCARD 0xff

enum provanity_cuda_mode
{
	/* EVM leading-nibble run: pattern[0] is the target nibble (0..15).
	 * Score equals the length of the maximum prefix of nibbles that all
	 * equal pattern[0]. */
	PROVANITY_CUDA_MODE_LEADING = 0,
	/* EVM positional pattern: pattern[i] is the target nibble for position
	 * i (0..15), or PROVANITY_CUDA_PATTERN_WILDCARD for a wildcard. The
	 * score is the number of concrete positions that match. */
	PROVANITY_CUDA_MODE_PATTERN = 1,
	/* Tron positional pattern: pattern[i] holds a base58 character target
	 * for position i (1..33), or 0 for wildcard / unused slot. Position 0
	 * is implicitly 'T' and ignored. */
	PROVANITY_CUDA_MODE_TRON_PATTERN = 2
};

enum provanity_cuda_event_type
{
	PROVANITY_CUDA_EVENT_READY = 1,
	PROVANITY_CUDA_EVENT_PROGRESS = 2,
	PROVANITY_CUDA_EVENT_FOUND = 3,
	PROVANITY_CUDA_EVENT_ERROR = 4,
	/* Lifecycle event fired before each long-running setup step inside
	 * provanity_cuda_run. Reuses existing fields to keep the event struct
	 * binary-stable: error_code holds a short phase tag (alloc / precomp /
	 * init_lanes / search_start), error_message holds a human-readable
	 * description, devices[0].id holds the device id the phase belongs to,
	 * and attempts holds a phase-specific numeric value (lane count, byte
	 * count) for callers that want to format their own message. */
	PROVANITY_CUDA_EVENT_PHASE = 5
};

typedef struct provanity_cuda_device
{
	int32_t id;
	char name[128];
	uint64_t global_mem;
	int32_t multiprocessors;
	int32_t compute_major;
	int32_t compute_minor;
	uint64_t hashrate;
} provanity_cuda_device;

typedef struct provanity_cuda_event
{
	int32_t type;
	uint64_t elapsed_sec;
	uint64_t elapsed_ms;
	uint64_t attempts;
	uint64_t hashrate;
	int32_t score;
	int32_t device_count;
	provanity_cuda_device devices[PROVANITY_CUDA_MAX_DEVICES];
	char offset[65];
	char address[41];
	char error_code[64];
	char error_message[256];
} provanity_cuda_event;

typedef int32_t(PROVANITY_CUDA_CALL *provanity_cuda_callback)(const provanity_cuda_event *event, void *user_data);

typedef struct provanity_cuda_config
{
	const char *public_key_hex;
	int32_t mode;
	int32_t contract;
	uint8_t pattern[PROVANITY_CUDA_PATTERN_LEN];
	int32_t device_ids[PROVANITY_CUDA_MAX_DEVICES];
	int32_t device_count;
	uint32_t batch_multiple;
	uint32_t progress_interval_ms;
	uint32_t work_size;
	uint8_t stop_score;
} provanity_cuda_config;

PROVANITY_CUDA_API int32_t PROVANITY_CUDA_CALL provanity_cuda_version(char *version, uint32_t version_len);
PROVANITY_CUDA_API int32_t PROVANITY_CUDA_CALL provanity_cuda_list_devices(provanity_cuda_device *devices, int32_t max_devices, char *error, uint32_t error_len);
PROVANITY_CUDA_API int32_t PROVANITY_CUDA_CALL provanity_cuda_run(const provanity_cuda_config *config, provanity_cuda_callback callback, void *user_data, char *error, uint32_t error_len);

#endif /* PROVANITY_CUDA_BACKEND_H */
