#pragma once

#include "sha256.cuh"

__constant__ const uint8_t g_base58_alphabet[58] = {
	'1', '2', '3', '4', '5', '6', '7', '8', '9',
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H',
	'J', 'K', 'L', 'M', 'N', 'P', 'Q', 'R',
	'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h',
	'i', 'j', 'k', 'm', 'n', 'o', 'p', 'q',
	'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'};

__device__ __forceinline__ void pv_tron_base58check(const uint8_t *const hash, uint8_t *const out)
{
	uint8_t payload[21];
	uint8_t checksum1[32];
	uint8_t checksum2[32];
	uint8_t input[25];
	uint8_t encoded[40];

	payload[0] = 0x41;
	for (int i = 0; i < 20; ++i)
	{
		payload[i + 1] = hash[i];
		input[i + 1] = hash[i];
	}
	pv_sha256_oneblock(payload, 21, checksum1);
	pv_sha256_oneblock(checksum1, 32, checksum2);

	input[0] = 0x41;
	for (int i = 0; i < 4; ++i)
	{
		input[21 + i] = checksum2[i];
	}

	int zeroes = 0;
	while (zeroes < 25 && input[zeroes] == 0)
	{
		++zeroes;
	}

	int encoded_len = 0;
	for (int start = zeroes; start < 25;)
	{
		uint32_t remainder = 0;
		for (int i = start; i < 25; ++i)
		{
			const uint32_t acc = (remainder << 8) | input[i];
			input[i] = (uint8_t)(acc / 58U);
			remainder = acc % 58U;
		}
		encoded[encoded_len++] = g_base58_alphabet[remainder];
		while (start < 25 && input[start] == 0)
		{
			++start;
		}
	}

	const int out_len = zeroes + encoded_len;
	int pad = 34 - out_len;
	if (pad < 0)
	{
		pad = 0;
	}
	for (int i = 0; i < 34; ++i)
	{
		out[i] = g_base58_alphabet[0];
	}
	for (int i = 0; i < zeroes && pad + i < 34; ++i)
	{
		out[pad + i] = g_base58_alphabet[0];
	}
	for (int i = 0; i < encoded_len && pad + zeroes + i < 34; ++i)
	{
		out[pad + zeroes + i] = encoded[encoded_len - 1 - i];
	}
}
