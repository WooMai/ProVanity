#pragma once

#include "provanity_cuda.h"
#include "types.cuh"

__constant__ const Felt256 g_mod = {{0xfffffc2f, 0xfffffffe, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff}};
__constant__ uint8_t g_pattern[PROVANITY_CUDA_PATTERN_LEN];

__device__ __forceinline__ Felt256 secp256k1_gx()
{
	return {{0x16f81798, 0x59f2815b, 0x2dce28d9, 0x029bfcdb, 0xce870b07, 0x55a06295, 0xf9dcbbac, 0x79be667e}};
}

__device__ __forceinline__ Felt256 secp256k1_gy()
{
	return {{0xfb10d4b8, 0x9c47d08f, 0xa6855419, 0xfd17b448, 0x0e1108a8, 0x5da4fbfc, 0x26a3c465, 0x483ada77}};
}
