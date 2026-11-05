#ifndef MIERU_JNI_H
#define MIERU_JNI_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Entries exported by libmieru.so (c-shared Go build).
int64_t mieruclient_new(void);
void mieruclient_free(int64_t handle);
int mieruclient_configure(int64_t handle, const char *serverAddress, const char *portSpec,
                          const char *username, const char *password,
                          int32_t mtu, const char *protocol);
int mieruclient_start(int64_t handle);
void mieruclient_stop(int64_t handle);
int mieruclient_is_running(int64_t handle);
int mieruclient_activate(int64_t handle);
void mieruclient_deactivate(void);
int mieruclient_dial(int32_t instanceNum, uint64_t token, uint64_t *dialIdOut,
                     const char *address, int32_t port, int32_t timeoutMillis);
void mieruclient_cancel_dial(uint64_t dialId);
const char *mieruclient_parse_url(const char *url);
const char *mieruclient_create_url(const char *serverAddress, const char *portSpec,
                                   const char *username, const char *password,
                                   int32_t mtu, const char *protocol);
void mieruclient_free_string(char *value);

// Registers the dial-result dispatch callbacks implemented in libtmessages.so.
// They are passed as raw function pointers so libmieru.so has no reverse
// symbol dependency (Android loads .so libraries with RTLD_LOCAL|RTLD_NOW).
void mieruclient_register_callbacks(uintptr_t connected, uintptr_t failed);

#ifdef __cplusplus
}
#endif

#endif
