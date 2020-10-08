/* libnbd golang handle.
 * Copyright (C) 2013-2020 Red Hat Inc.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */

package libnbd

/*
#cgo LDFLAGS: -lnbd
#cgo CFLAGS: -D_GNU_SOURCE=1

#include <stdio.h>
#include <stdlib.h>

#include "libnbd.h"
#include "wrappers.h"

*/
import "C"

import "unsafe"

/* Asynchronous I/O buffer. */
type AioBuffer struct {
	P    unsafe.Pointer
	Size uint
}

func MakeAioBuffer(size uint) AioBuffer {
	return AioBuffer{C.malloc(C.ulong(size)), size}
}

func FromBytes(buf []byte) AioBuffer {
	size := len(buf)
	ret := MakeAioBuffer(uint(size))
	for i := 0; i < len(buf); i++ {
		*ret.Get(uint(i)) = buf[i]
	}
	return ret
}

func (b *AioBuffer) Free() {
	C.free(b.P)
}

func (b *AioBuffer) Bytes() []byte {
	return C.GoBytes(b.P, C.int(b.Size))
}

func (b *AioBuffer) Get(i uint) *byte {
	return (*byte)(unsafe.Pointer(uintptr(b.P) + uintptr(i)))
}
