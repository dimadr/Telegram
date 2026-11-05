// Copyright (C) 2024  mieru authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// cgo entry points for the mieru Android client.
//
// All logic lives in mierucli.go (pure Go). This file only marshals C types.
// The library is built with -buildmode=c-shared; tgnet (libtmessages.so) calls
// mieruclient_dial/mieruclient_cancel_dial and registers its dial-result
// callbacks as plain function pointers (no reverse symbol dependency on
// libtmessages.so), which is required because Android loads .so files with
// RTLD_LOCAL|RTLD_NOW.
package main

/*
#include <stdint.h>
#include <stddef.h>
#include <stdlib.h>

typedef void (*mieru_connected_fn)(int32_t instanceNum, uint64_t token, uint64_t dialId, int32_t fd);
typedef void (*mieru_failed_fn)(int32_t instanceNum, uint64_t token, uint64_t dialId, int32_t error);

void mierujni_register_callbacks(uintptr_t connected, uintptr_t failed);
void mierujni_call_connected(int32_t instanceNum, uint64_t token, uint64_t dialId, int32_t fd);
void mierujni_call_failed(int32_t instanceNum, uint64_t token, uint64_t dialId, int32_t error);
*/
import "C"

import (
	"unsafe"
)

func init() {
	// C++ (TgNetWrapper) registers its dispatch callbacks through
	// mieruclient_register_callbacks; these trampolines forward Go goroutine
	// results into the registered function pointers.
	setHooks(func(instanceNum int32, token uint64, dialID uint64, fd int32) {
		C.mierujni_call_connected(C.int32_t(instanceNum), C.uint64_t(token), C.uint64_t(dialID), C.int32_t(fd))
	}, func(instanceNum int32, token uint64, dialID uint64, errorCode int32) {
		C.mierujni_call_failed(C.int32_t(instanceNum), C.uint64_t(token), C.uint64_t(dialID), C.int32_t(errorCode))
	})
}

//export mieruclient_register_callbacks
func mieruclient_register_callbacks(connected C.uintptr_t, failed C.uintptr_t) {
	C.mierujni_register_callbacks(connected, failed)
}

//export mieruclient_new
func mieruclient_new() C.int64_t {
	return C.int64_t(appNewClient())
}

//export mieruclient_free
func mieruclient_free(handle C.int64_t) {
	appFreeClient(int64(handle))
}

//export mieruclient_configure
func mieruclient_configure(handle C.int64_t, serverAddress *C.char, portSpec *C.char, username *C.char, password *C.char, mtu C.int32_t, protocolStr *C.char) C.int {
	if serverAddress == nil || portSpec == nil || username == nil || password == nil || protocolStr == nil {
		return 0
	}
	ok := appConfigure(int64(handle), C.GoString(serverAddress), C.GoString(portSpec), C.GoString(username), C.GoString(password), int32(mtu), C.GoString(protocolStr))
	if ok {
		return 1
	}
	return 0
}

//export mieruclient_start
func mieruclient_start(handle C.int64_t) C.int {
	if appStart(int64(handle)) {
		return 1
	}
	return 0
}

//export mieruclient_stop
func mieruclient_stop(handle C.int64_t) {
	appStop(int64(handle))
}

//export mieruclient_is_running
func mieruclient_is_running(handle C.int64_t) C.int {
	if appIsRunning(int64(handle)) {
		return 1
	}
	return 0
}

//export mieruclient_activate
func mieruclient_activate(handle C.int64_t) C.int {
	if appActivate(int64(handle)) {
		return 1
	}
	return 0
}

//export mieruclient_deactivate
func mieruclient_deactivate() {
	appDeactivate()
}

//export mieruclient_dial
func mieruclient_dial(instanceNum C.int32_t, token C.uint64_t, dialIdOut *C.uint64_t, address *C.char, port C.int32_t, timeoutMillis C.int32_t) C.int {
	if address == nil || dialIdOut == nil {
		return 0
	}
	dialID, ok := appDial(int32(instanceNum), uint64(token), C.GoString(address), int(port), int32(timeoutMillis))
	if !ok {
		return 0
	}
	*dialIdOut = C.uint64_t(dialID)
	return 1
}

//export mieruclient_parse_url
func mieruclient_parse_url(rawURL *C.char) *C.char {
	if rawURL == nil {
		return nil
	}
	result := appParseURL(C.GoString(rawURL))
	if result == "" {
		return nil
	}
	return C.CString(result)
}

//export mieruclient_create_url
func mieruclient_create_url(serverAddress *C.char, portSpec *C.char, username *C.char, password *C.char, mtu C.int32_t, protocolStr *C.char) *C.char {
	if serverAddress == nil || portSpec == nil || username == nil || password == nil || protocolStr == nil {
		return nil
	}
	result := appCreateURL(C.GoString(serverAddress), C.GoString(portSpec), C.GoString(username), C.GoString(password), int32(mtu), C.GoString(protocolStr))
	if result == "" {
		return nil
	}
	return C.CString(result)
}

//export mieruclient_free_string
func mieruclient_free_string(value *C.char) {
	C.free(unsafe.Pointer(value))
}

//export mieruclient_cancel_dial
func mieruclient_cancel_dial(dialID C.uint64_t) {
	appCancelDial(uint64(dialID))
}
