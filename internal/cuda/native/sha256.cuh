#pragma once

#include "types.cuh"

__constant__ const uint32_t g_sha256_k[64] = {
	0x428a2f98U, 0x71374491U, 0xb5c0fbcfU, 0xe9b5dba5U,
	0x3956c25bU, 0x59f111f1U, 0x923f82a4U, 0xab1c5ed5U,
	0xd807aa98U, 0x12835b01U, 0x243185beU, 0x550c7dc3U,
	0x72be5d74U, 0x80deb1feU, 0x9bdc06a7U, 0xc19bf174U,
	0xe49b69c1U, 0xefbe4786U, 0x0fc19dc6U, 0x240ca1ccU,
	0x2de92c6fU, 0x4a7484aaU, 0x5cb0a9dcU, 0x76f988daU,
	0x983e5152U, 0xa831c66dU, 0xb00327c8U, 0xbf597fc7U,
	0xc6e00bf3U, 0xd5a79147U, 0x06ca6351U, 0x14292967U,
	0x27b70a85U, 0x2e1b2138U, 0x4d2c6dfcU, 0x53380d13U,
	0x650a7354U, 0x766a0abbU, 0x81c2c92eU, 0x92722c85U,
	0xa2bfe8a1U, 0xa81a664bU, 0xc24b8b70U, 0xc76c51a3U,
	0xd192e819U, 0xd6990624U, 0xf40e3585U, 0x106aa070U,
	0x19a4c116U, 0x1e376c08U, 0x2748774cU, 0x34b0bcb5U,
	0x391c0cb3U, 0x4ed8aa4aU, 0x5b9cca4fU, 0x682e6ff3U,
	0x748f82eeU, 0x78a5636fU, 0x84c87814U, 0x8cc70208U,
	0x90befffaU, 0xa4506cebU, 0xbef9a3f7U, 0xc67178f2U};

#define CUDA_SHA256_ROTR(x, n) (((x) >> (n)) | ((x) << (32U - (n))))
#define CUDA_SHA256_CH(x, y, z) (((x) & (y)) ^ (~(x) & (z)))
#define CUDA_SHA256_MAJ(x, y, z) (((x) & (y)) ^ ((x) & (z)) ^ ((y) & (z)))
#define CUDA_SHA256_EP0(x) (CUDA_SHA256_ROTR((x), 2) ^ CUDA_SHA256_ROTR((x), 13) ^ CUDA_SHA256_ROTR((x), 22))
#define CUDA_SHA256_EP1(x) (CUDA_SHA256_ROTR((x), 6) ^ CUDA_SHA256_ROTR((x), 11) ^ CUDA_SHA256_ROTR((x), 25))
#define CUDA_SHA256_SIG0(x) (CUDA_SHA256_ROTR((x), 7) ^ CUDA_SHA256_ROTR((x), 18) ^ ((x) >> 3))
#define CUDA_SHA256_SIG1(x) (CUDA_SHA256_ROTR((x), 17) ^ CUDA_SHA256_ROTR((x), 19) ^ ((x) >> 10))

__device__ __forceinline__ void pv_sha256_oneblock(const uint8_t *const message, const uint32_t len, uint8_t *const out)
{
	uint32_t w[64];
	const uint64_t bit_len = ((uint64_t)len) * 8UL;

	for (uint32_t i = 0; i < 16; ++i)
	{
		uint32_t word = 0;
		for (uint32_t j = 0; j < 4; ++j)
		{
			const uint32_t index = i * 4 + j;
			uint8_t b = 0;
			if (index < len)
			{
				b = message[index];
			}
			else if (index == len)
			{
				b = 0x80;
			}
			else if (index >= 56)
			{
				const uint32_t shift = (63U - index) * 8U;
				b = (uint8_t)((bit_len >> shift) & 0xffUL);
			}
			word = (word << 8) | b;
		}
		w[i] = word;
	}
	for (uint32_t i = 16; i < 64; ++i)
	{
		w[i] = CUDA_SHA256_SIG1(w[i - 2]) + w[i - 7] + CUDA_SHA256_SIG0(w[i - 15]) + w[i - 16];
	}

	uint32_t a = 0x6a09e667U;
	uint32_t b = 0xbb67ae85U;
	uint32_t c = 0x3c6ef372U;
	uint32_t d = 0xa54ff53aU;
	uint32_t e = 0x510e527fU;
	uint32_t f = 0x9b05688cU;
	uint32_t g = 0x1f83d9abU;
	uint32_t h = 0x5be0cd19U;

	for (uint32_t i = 0; i < 64; ++i)
	{
		const uint32_t t1 = h + CUDA_SHA256_EP1(e) + CUDA_SHA256_CH(e, f, g) + g_sha256_k[i] + w[i];
		const uint32_t t2 = CUDA_SHA256_EP0(a) + CUDA_SHA256_MAJ(a, b, c);
		h = g;
		g = f;
		f = e;
		e = d + t1;
		d = c;
		c = b;
		b = a;
		a = t1 + t2;
	}

	const uint32_t state[8] = {
		0x6a09e667U + a,
		0xbb67ae85U + b,
		0x3c6ef372U + c,
		0xa54ff53aU + d,
		0x510e527fU + e,
		0x9b05688cU + f,
		0x1f83d9abU + g,
		0x5be0cd19U + h};
	for (uint32_t i = 0; i < 8; ++i)
	{
		out[i * 4] = (uint8_t)(state[i] >> 24);
		out[i * 4 + 1] = (uint8_t)(state[i] >> 16);
		out[i * 4 + 2] = (uint8_t)(state[i] >> 8);
		out[i * 4 + 3] = (uint8_t)(state[i]);
	}
}
